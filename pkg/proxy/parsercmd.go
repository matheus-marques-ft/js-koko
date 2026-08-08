package proxy

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"unicode"

	"github.com/LeeEirc/terminalparser"
	"github.com/jumpserver/koko/pkg/logger"
)

var terminalDebug = false

func init() {
	if os.Getenv("TERMINALPARSER") != "" {
		terminalDebug = true
	}
}

const (
	LinuxScreen = iota + 1
	UsqlScreen
	MongoScreen
	TmuxScreen
	WindowsScreen
)

func DefaultEnterKeyPressHandler(p []byte) bool {
	return bytes.ContainsRune(p, '\r')
}

const maxBufSize = 1024 * 100

const (
	InputPreState = iota + 1
	InputState
	InVimState
	OutputState
)

type ScreenParser interface {
	Feed([]byte)
	GetCursorRow() string
}

type TerminalParser struct {
	InputBuf bytes.Buffer
	Ps1sStr  string
	Screen   *terminalparser.Screen
	state    int
	once     sync.Once
	mux      sync.Mutex

	IsEnter func(p []byte) bool
	cmd     string

	commands []string

	EmitCommands func(cmd, out string)

	tmuxParser *terminalparser.TmuxParser
	isSubMode  bool

	srvOutputBuf bytes.Buffer

	screenType    int
	preScreenType int
	//screenParser ScreenParser

	winScreenParser   *terminalparser.WindowsParser
	mongoScreenParser *terminalparser.MongoShParser
	usqlScreenParser  *terminalparser.USqlParser
}

func (s *TerminalParser) SetState(state int) {
	s.state = state
}

func (s *TerminalParser) CheckSubScreen(b []byte) {
	if !s.isSubMode && IsEditEnterMode(b) {
		s.isSubMode = true
		s.tmuxParser = terminalparser.NewTmuxParser()
		s.screenType = TmuxScreen
	}
	if s.isSubMode && IsEditExitMode(b) {
		s.isSubMode = false
		s.tmuxParser = nil
		s.srvOutputBuf.Reset()
		s.screenType = s.preScreenType
	}
}

func (s *TerminalParser) resetCommand() {
	s.cmd = ""
	s.commands = nil
}

func (s *TerminalParser) GetCursorRow() string {
	switch s.screenType {
	case LinuxScreen:
		row := s.Screen.GetCursorRow()
		return row.String()
	case UsqlScreen:
		row := s.usqlScreenParser.TmuxScreen.GetCursorRow()
		return row.String()
	case MongoScreen:
		row := s.mongoScreenParser.TmuxScreen.GetCursorRow()
		return row.String()
	case TmuxScreen:
		row := s.tmuxParser.TmuxScreen.GetCursorRow()
		return row.String()
	default:
		row := s.Screen.GetCursorRow()
		return row.String()
	}
}

func (s *TerminalParser) feed(p []byte) {
	defer func() {
		if r := recover(); r != nil {
			if terminalDebug {
				fmt.Printf("Recovered from panic: %s %s\n", r, string(debug.Stack()))
			}
		}
	}()

	switch s.screenType {
	case UsqlScreen:
		s.usqlScreenParser.Feed(p)
	case MongoScreen:
		s.mongoScreenParser.Feed(p)
	case TmuxScreen:
		s.tmuxParser.Feed(p)
	//case LinuxScreen:
	//	s.Screen.Feed(p)
	//	s.ResizeRows()
	default:
		// Defaults to LinuxScreen
		s.Screen.Feed(p)
		s.ResizeRows()
	}
	if terminalDebug {
		fmt.Println("---------Feed-------------")
		fmt.Println(hex.Dump(p))
		fmt.Println("current row: ", s.GetCursorRow())
		fmt.Println()
	}
}

func (s *TerminalParser) Feed(p []byte) {
	defer func() {
		if r := recover(); r != nil {
			if terminalDebug {
				fmt.Printf("Recovered from panic: %s %s\n", r, string(debug.Stack()))
			}
		}
	}()
	s.mux.Lock()
	defer s.mux.Unlock()
	// Check whether this is a tmux or screen situation
	s.CheckSubScreen(p)

	s.feed(p)

	if s.state == OutputState {
		// Only write to output once cmd has been parsed out, to reduce memory usage
		if s.srvOutputBuf.Len() < maxBufSize {
			s.srvOutputBuf.Write(p)
		} else {
			// Output has run for a long time and reached the max size, finalize the command immediately
			outputBuf := s.TrySrvOutput()
			if s.EmitCommands != nil {
				s.EmitCommands(s.cmd, outputBuf)
				s.cmd = ""
				return
			}
		}
		ps1 := s.Ps1sStr
		half := len(ps1) / 2
		halfPs1 := ps1[:half]
		rowStr := s.GetCursorRow()
		// Single-line command parsing
		if strings.HasPrefix(rowStr, halfPs1) && s.cmd != "" {
			outputBuf := s.TrySrvOutput()
			if s.EmitCommands != nil {
				s.EmitCommands(s.cmd, outputBuf)
			}
			if terminalDebug {
				// From here, find the previous matching ps1 row; the rows in between are the output
				fmt.Println("============= match ps1 command================")
				fmt.Println("ps1: ", s.Ps1sStr)
				fmt.Println("command input:  ", s.cmd)
				fmt.Println("command output: ", outputBuf)
				fmt.Println("===============================================")
				// At this point it should be in input state, the command has ended
			}
			s.cmd = ""
			return
		}

		// Parsing multi-line commands must wait for the full output; the data is parsed on the next input's result. See the handling of len(s.commands) >= 1 in WriteInput.
	}
}

func (s *TerminalParser) OnSize() {

}

func (s *TerminalParser) TrySrvOutput() string {
	output := s.srvOutputBuf.Bytes()
	if s.tmuxParser != nil {
		output = tmuxBar2Regx.ReplaceAll(output, []byte{})
	}
	outputs := terminalparser.ParseOutput(output)
	var str strings.Builder
	ps1 := strings.TrimSpace(s.Ps1sStr)
	for _, o := range outputs {
		o = strings.TrimSpace(o)
		o = strings.ReplaceAll(o, ps1, "")
		if len(o) > 0 && str.Len() < maxBufSize {
			str.WriteString(o)
			str.WriteString("\n")
		}
	}
	s.srvOutputBuf.Reset()
	if s.srvOutputBuf.Cap() > maxBufSize {
		s.srvOutputBuf = bytes.Buffer{}
	}
	return str.String()
}

func (s *TerminalParser) TryOutput() string {
	s.cmd = ""
	return s.TrySrvOutput()
}

func (s *TerminalParser) ResizeRows() {
	rowsLen := len(s.Screen.Rows)
	if rowsLen >= 2000 {
		newRows := make([]*terminalparser.Row, 1000, 2000)
		oldRows := s.Screen.Rows
		oldY := s.Screen.Cursor.Y
		keep := 1000
		start := rowsLen - keep
		if start < 0 {
			start = 0
		}
		latestRows := oldRows[start:]
		copy(newRows, latestRows)
		s.Screen.Rows = newRows
		if oldY >= len(latestRows) {
			s.Screen.Cursor.Y = len(latestRows)
		}
		// for gc
		for i := 0; i < start; i++ {
			oldRows[i] = nil
		}
		// for gc
		oldRows = nil
		latestRows = nil
		logger.Debugf("Resize Y: %d, row Len: %d", s.Screen.Cursor.Y, len(s.Screen.Rows))
	}
}

func IsPrintable(s string) bool {
	for _, r := range s {
		switch r {
		case '\t', '\n', '\r':
			continue
		default:
		}
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

func (s *TerminalParser) WriteInput(chars []byte) (string, bool) {
	if len(chars) == 0 {
		return "", false
	}
	s.mux.Lock()
	defer s.mux.Unlock()

	s.once.Do(func() {
		s.state = InputState
		s.Ps1sStr = s.GetPs1()
	})

	// Copy-pasted multi-line command execution
	s.TryMultipleCommands()

	isEnterFunc := DefaultEnterKeyPressHandler
	if s.IsEnter != nil {
		isEnterFunc = s.IsEnter
	}

	/*
		If it is a multi-line command, first fully parse the input content for interception;
		the actual executed command and its result are then looked up from the command parser.
	*/
	s.InputBuf.Write(chars)
	if isEnterFunc(chars) {
		inputStr := strings.TrimSpace(s.InputBuf.String())
		s.state = OutputState
		//if s.isSubMode {
		//	cmd = s.TryTmuxInput()
		//} else {
		//	// For multi-line commands, starting from the latest row, everything found going backward up to the most recent ps1 is the command
		//	cmd = s.TryInput()
		//}
		cmd := s.TryLastRowInput()
		if cmd == "" && len(chars) > 1 {
			// Parsing from the return value, when cmd is empty the current input is used instead
			cmd = strings.TrimSpace(string(chars))
			if strings.Contains(cmd, "\r") {
				// Multi-line command
				s.commands = strings.Split(cmd, "\r")
			}
		} else {
			s.cmd = cmd
			suffixCmd := cmd[len(cmd)/2:]
			if IsPrintable(inputStr) {
				if strings.Contains(inputStr, suffixCmd) {
					cmd = inputStr
				} else if strings.Contains(inputStr, "\r") {
					s.commands = strings.Split(inputStr, "\r")
					cmd = inputStr
				}
			}
			if s.cmd == "" && cmd != "" && len(s.commands) == 0 {
				if IsPasswordPrompt(s.Ps1sStr) {
					if terminalDebug {
						fmt.Println("============ password Input ignore =============")
						fmt.Println("ps1: ", s.Ps1sStr)
						fmt.Println("inputStr:", inputStr)
					}
					cmd = ""
					s.cmd = cmd
				}
			}
		}
		if terminalDebug {
			// From here, find the previous matching ps1 row; the rows in between are the output
			fmt.Println("============= enter command================")
			fmt.Println("ps1: ", s.Ps1sStr)
			fmt.Println("command input1:  ", cmd)
			fmt.Println("command input2:  ", s.cmd)
			fmt.Println("commands :  ", s.commands)
			fmt.Println("===============================================")
			// At this point it should be in output state, the command has ended
		}
		return cmd, true
	}
	if s.state == OutputState {
		s.state = InputState
		s.Ps1sStr = s.GetPs1()
	}
	return "", false
}

func (s *TerminalParser) TryTmuxInput() string {
	lastLine := s.tmuxParser.TmuxScreen.GetCursorRow()
	cmd := strings.TrimPrefix(lastLine.String(), s.Ps1sStr)
	s.InputBuf.Reset()
	return strings.TrimSpace(cmd)
}

func (s *TerminalParser) TryInput() string {
	lastLine := s.Screen.GetCursorRow()
	cmd := strings.TrimPrefix(lastLine.String(), s.Ps1sStr)
	s.InputBuf.Reset()
	return strings.TrimSpace(cmd)
}

func (s *TerminalParser) TryLastRowInput() string {
	rowStr := s.GetCursorRow()
	cmd := strings.TrimPrefix(rowStr, s.Ps1sStr)
	s.InputBuf.Reset()
	return strings.TrimSpace(cmd)
}

func (s *TerminalParser) GetPs1() string {
	rowStr := s.GetCursorRow()
	return strings.TrimSuffix(rowStr, s.InputBuf.String())
}

func (s *TerminalParser) FindCommands(cmds []string, startCmd string) {
	// Search for commands backward starting from the last row
	outputs := make([]string, 0, 10)
	rows := s.Screen.Rows
	j := len(rows) - 1

	// Remove interference from startCMd
	for j > 0 {
		row := rows[j]
		j--
		if strings.Contains(row.String(), startCmd) {
			break
		}
	}
	ps1 := s.Ps1sStr
	half := len(ps1) / 2
	halfPs1 := ps1[:half]
	if terminalDebug {
		fmt.Println("ps1: ", ps1, " halfPs1: ", halfPs1)
	}
	for i := len(cmds) - 1; i >= 0; i-- {
		currentCommand := cmds[i]
		if currentCommand == "" {
			continue
		}
		for j > 0 {
			row := rows[j]
			rowStr := row.String()
			j--
			if strings.Contains(rowStr, currentCommand) && strings.Contains(rowStr, halfPs1) {
				// Matched the current command, get all the output below it
				output := reverseString(outputs)
				if s.EmitCommands != nil {
					s.EmitCommands(currentCommand, output)
					if terminalDebug {
						fmt.Println("-----------EmitCommands----------- ")
						fmt.Println("command input:  ", currentCommand)
						fmt.Println("command output: ", output)
					}
				}
				outputs = make([]string, 0, 10)
				break
			}
			outputStr := strings.TrimPrefix(rowStr, s.Ps1sStr)
			if outputStr != "" {
				outputs = append(outputs, outputStr)
			}
		}
	}
}

func (s *TerminalParser) TryMultipleCommands() {
	if s.screenType != LinuxScreen {
		// Only supported for the linux screen mode
		return
	}
	if len(s.commands) >= 1 {
		commands := s.commands

		// Need to obtain the current command result from the returned data
		lastCommand := commands[len(commands)-1]
		startCommand := lastCommand
		if startCommand == "" {
			startCommand = s.Ps1sStr
		} else {
			// Exclude the last one, which has not been executed yet
			commands = commands[:len(commands)-1]
		}
		if terminalDebug {
			for i := len(commands) - 1; i >= 0; i-- {
				cmd := commands[i]
				fmt.Printf("may be command: `%s`\n", cmd)
			}
		}
		s.FindCommands(commands, startCommand)
		s.commands = nil
	}
}

func reverseString(rows []string) string {
	var str strings.Builder

	for i := len(rows) - 1; i >= 0; i-- {
		str.WriteString(rows[i])
		str.Write([]byte{'\r', '\n'})
	}
	return str.String()
}

// filtering for password input scenarios
var passwordPromptRegexps = []*regexp.Regexp{
	regexp.MustCompile(`(?i)password:?$`),                    // Common "Password:"
	regexp.MustCompile(`(?i)\[sudo]\s*password\s*for\s+.*:`), // [sudo] password for user:
	regexp.MustCompile(`(?i)enter\s+passphrase\s+for\s+.*:`), // SSH/GPG private key passphrase
	regexp.MustCompile(`(?i)passphrase\s+for\s+key\s+.*:`),   // git/ssh key prompt
	regexp.MustCompile(`(?i)请输入密码[:：]?$`),
	regexp.MustCompile(`(?i)mot de passe[:：]?$`),
	regexp.MustCompile(`(?i)contraseña[:：]?$`),
	regexp.MustCompile(`(?i)senha[:：]?$`),
}

func IsPasswordPrompt(ps1 string) bool {
	ps1 = strings.TrimSpace(ps1)
	for _, re := range passwordPromptRegexps {
		if re.MatchString(ps1) {
			return true
		}
	}
	return false
}

// Combined regular expression, matching the following four patterns:
// 1. Hide cursor: ESC[?25l
// 2. ANSI color escape sequence: ESC[numberm
// 3. ANSI position escape sequence: ESC[number;numberH
// 4. Status bar format starting with a number: [number] space content space content...
// 0D 0A \r \n
var (
	tmuxBarRegx = regexp.MustCompile(`\x1b\[\?(\d+)l\x1b\[(\d+)m\x1b\[(\d+)m\x1b\[(\d+);(\d+)H\[(\d+)]\s+\d+:.+\s+.+\s+.+\s+.+\x1b\(B.*\x1b\[\?(\d+)l\x1b\[\?(\d+)h`)
	// \[(\d+)]\s+\d+:.+\s+.+\s+.+\s+.+

	// May contain \r\n
	//tmuxBar1Regx = regexp.MustCompile(`\r\n\[(\d+)]\s+\d+:.+\s+.+\s+.+\s+.+\x1b\(B`)

	// Does not contain \r\n
	tmuxBar2Regx = regexp.MustCompile(`\[(\d+)]\s+\d+:.+\s+.+\s+.+\s+.+\x1b\(B`)
)
