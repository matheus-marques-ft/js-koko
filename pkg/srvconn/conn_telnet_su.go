package srvconn

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"time"

	"github.com/jumpserver/koko/pkg/logger"
)

func LoginToTelnetSu(sc *TelnetConnection) error {
	cfg := sc.cfg.suCfg
	suService, err := NewSuService(cfg, sc)
	if err != nil {
		return err
	}
	return suService.RunSwitchUser()
}

/*

Execution flow for switching users

I. The switch-user execution flow differs by system
	Linux sudo execution flow
	1. Execute su - username; exit (the exit here is to exit sudo)
	2. Wait for the password input prompt (if switching from root to a normal user, it may switch successfully right away)

	Cisco switch execution flow
	1. Execute enable
	2. Wait for the password input prompt

	Huawei switch execution flow
	1. Execute super 15 (the 15 here is the user privilege level)
	2. Wait for the password input prompt

	H3C switch execution flow
	1. Execute super level-15 (the 15 here is the user privilege level)
	2. Wait for the username prompt
	3. Wait for the password prompt

II. Wait to match the success prompt characters; if the failure prompt characters are matched instead, return a password error failure
III. If successful, return the switch prompt message, terminated with \r

About the success prompt:
The success prompt for Linux and Cisco switches contains
 Huawei:  [root@HUAWEI-xxx]


*/

func NewSuService(cfg *SuConfig, srv io.ReadWriteCloser) (*SuSwitchService, error) {
	successReg, err := regexp.Compile(cfg.SuccessPattern())
	if err != nil {
		return nil, fmt.Errorf("success pattern %s compile failed: %s", cfg.SuccessPattern(), err)
	}
	passwordReg, err := regexp.Compile(cfg.PasswordMatchPattern())
	if err != nil {
		return nil, fmt.Errorf("password pattern %s compile failed: %s", cfg.PasswordMatchPattern(), err)
	}
	usernameReg, err := regexp.Compile(cfg.UsernameMatchPattern())
	if err != nil {
		return nil, fmt.Errorf("username pattern %s compile failed: %s", cfg.UsernameMatchPattern(), err)
	}
	failedPattern := createFailedPattern()
	failedReg, err := regexp.Compile(failedPattern)
	if err != nil {
		return nil, fmt.Errorf("failed pattern %s compile failed: %s", failedPattern, err)
	}
	suService := SuSwitchService{
		cfg:            cfg,
		SrvConn:        srv,
		successRegexp:  successReg,
		usernameRegexp: usernameReg,
		passwordRegexp: passwordReg,
		failureRegexp:  failedReg,
	}
	return &suService, nil
}

type SuSwitchService struct {
	cfg         *SuConfig
	execCommand func()

	SrvConn io.ReadWriteCloser

	successRegexp  *regexp.Regexp
	usernameRegexp *regexp.Regexp
	passwordRegexp *regexp.Regexp
	failureRegexp  *regexp.Regexp

	inputAuthOnce bool
	needAuthOnce  bool
}

func (s *SuSwitchService) RunSwitchUser() error {
	s.runSwitchCommand()
	resultChan := make(chan error, 1)
	go s.loginUsernameOrPassword(resultChan)
	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()
	select {
	case ret := <-resultChan:
		return ret
	case <-ticker.C:
	}
	return ErrorTimeout
}

func (s *SuSwitchService) runSwitchCommand() {
	if s.execCommand != nil {
		s.execCommand()
	} else {
		cmd := s.cfg.SuCommand()
		_, _ = s.SrvConn.Write([]byte(cmd + "\r"))
		s.needAuthOnce = true
	}
}

func (s *SuSwitchService) loginUsernameOrPassword(resultChan chan<- error) {
	buf := make([]byte, 8192)
	var recStr bytes.Buffer
	for {
		nr, err2 := s.SrvConn.Read(buf)
		if err2 != nil {
			resultChan <- err2
			return
		}
		recStr.Write(buf[:nr])
		status := s.handleResult(recStr.Bytes())
		switch status {
		case StatusSuccess:
			// After success, end the switch
			resultChan <- nil
			return
		case StatusMatch:
			// Matched, clear the buffer
			recStr.Reset()
			logger.Debug("Sudo step result matched and rest")
			continue
		case StatusFailed:
			resultChan <- fmt.Errorf("failed login: %s", recStr.String())
			return
		case StatusUnMatch:
		default:

		}
		logger.Debugf("Sudo step result do not match any: %s", recStr.String())
		// No match, keep waiting
		time.Sleep(time.Millisecond * 100)
	}
}

func (s *SuSwitchService) handleResult(p []byte) matchStatus {
	newBytes := bytes.ReplaceAll(p, []byte("\r"), []byte("\n"))
	newBytes = bytes.ReplaceAll(newBytes, []byte("\n\n"), []byte("\n"))
	lineBytes := bytes.Split(newBytes, []byte("\n"))

	if s.usernameRegexp != nil && s.usernameRegexp.Match(p) {
		for _, line := range lineBytes {
			if s.usernameRegexp.Match(line) {
				_, _ = s.SrvConn.Write([]byte(s.cfg.SudoUsername + "\r"))
				logger.Debugf("Su switch step username pattern ok: %s", p)
				return StatusMatch
			}
		}
	}
	if s.passwordRegexp != nil {
		for _, line := range lineBytes {
			if s.passwordRegexp.Match(line) {
				if s.inputAuthOnce {
					logger.Debugf("Su switch step password pattern matched again: %s", p)
					return StatusFailed
				}
				_, _ = s.SrvConn.Write([]byte(s.cfg.SudoPassword + "\r"))
				s.inputAuthOnce = true
				logger.Debugf("Su switch step password pattern ok: %s", p)
				return StatusMatch
			}
		}
	}
	if s.needAuthOnce && s.inputAuthOnce {
		if s.failureRegexp != nil {
			for _, line := range lineBytes {
				if s.failureRegexp.Match(line) {
					logger.Debugf("Su switch step failed pattern ok: %s", p)
					return StatusFailed
				}
			}
		}
	}
	if s.successRegexp != nil {
		if s.needAuthOnce && !s.inputAuthOnce {
			logger.Debug("Su switch step need auth once but not input password")
			return StatusUnMatch
		}
		for _, line := range lineBytes {
			if s.successRegexp.Match(line) {
				logger.Debugf("Su switch step success pattern ok: %s", p)
				return StatusSuccess
			}
		}
	}
	return StatusUnMatch
}

type matchStatus int

const (
	StatusUnMatch matchStatus = 1
	StatusMatch   matchStatus = 2
	StatusSuccess matchStatus = 3
	StatusFailed  matchStatus = 4
)
