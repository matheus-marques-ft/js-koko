package srvconn

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/jumpserver-dev/sdk-go/service"

	"github.com/jumpserver/koko/pkg/common"
	"github.com/jumpserver/koko/pkg/config"
	"github.com/jumpserver/koko/pkg/localcommand"
	"github.com/jumpserver/koko/pkg/logger"
	"github.com/jumpserver/koko/pkg/utils"
)

var (
	ErrValidToken = errors.New("invalid token")

	_ ServerConnection = (*K8sCon)(nil)
)

const (
	k8sInitFilename = "init-kubectl.sh"
)

// Similar to `kubectl --insecure-skip-tls-verify=%s --token=%s --server=%s auth can-i get pods`

func IsValidK8sUserToken(k8sCfg *rest.Config) bool {
	client, err := kubernetes.NewForConfig(k8sCfg)
	if err != nil {
		logger.Errorf("K8sCon new config err: %s", err)
		return false
	}
	sar := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Verb:     "get",
				Resource: "pods",
			},
		},
	}
	authClient := client.AuthorizationV1()
	resp, err2 := authClient.SelfSubjectAccessReviews().Create(context.TODO(), sar, metav1.CreateOptions{})
	if err2 != nil {
		logger.Errorf("K8sCon check token pods auth err: %s", err2)
		return false
	}
	logger.Debugf("K8sCon check token pods auth resp: %+v", resp)
	return true
}

func NewK8sConnection(ops ...K8sOption) (*K8sCon, error) {
	if !config.GetConf().KokoPrivileged {
		return nil, ErrK8sNoPrivileged
	}

	args := &k8sOptions{
		Username:      os.Getenv("USER"),
		ClusterServer: "https://127.0.0.1:8443",
		Token:         "",
		IsSkipTls:     true,
		ExtraEnv:      map[string]string{},
	}
	for _, setter := range ops {
		setter(args)
	}

	k8sCfg := args.K8sCfg()
	if !IsValidK8sUserToken(k8sCfg) {
		return nil, ErrValidToken
	}
	kubeProxy := NewKubectlProxyConn(args)
	err := kubeProxy.Start()
	if err != nil {
		logger.Errorf("K8sCon start proxy err: %s", err)
		return nil, fmt.Errorf("K8sCon start proxy err: %w", err)
	}

	var fifoPath string
	var fifoFile *os.File
	if args.JMService != nil && args.TokenID != "" {
		fifoPath, fifoFile, err = createAliasFifo()
		if err != nil {
			logger.Errorf("K8sCon create alias fifo err: %s, disabling alias saving for this session", err)
			fifoPath, fifoFile = "", nil
		}
	}
	args.AliasFifoPath = fifoPath

	envs := kubeProxy.Env()
	lcmd, err := startK8SLocalCommand(envs)
	if err != nil {
		if fifoFile != nil {
			_ = fifoFile.Close()
		}
		if fifoPath != "" {
			_ = os.Remove(fifoPath)
		}
		logger.Errorf("K8sCon start local pty err: %s", err)
		return nil, fmt.Errorf("K8sCon start local pty err: %w", err)
	}
	err = lcmd.SetWinSize(args.win.Width, args.win.Height)
	if err != nil {
		_ = lcmd.Close()
		return nil, err
	}

	if fifoFile != nil {
		go syncPendingK8sAliases(fifoFile, args.JMService, args.TokenID)
	}
	go watchK8sSandboxIsolation(lcmd)

	return &K8sCon{
		options:      args,
		LocalCommand: lcmd,
		proxy:        kubeProxy,
		fifoPath:     fifoPath,
		fifoFile:     fifoFile,
	}, nil
}

type K8sCon struct {
	proxy    *KubectlProxyConn
	options  *k8sOptions
	fifoPath string
	fifoFile *os.File
	*localcommand.LocalCommand
}

func (k *K8sCon) KeepAlive() error {
	return nil
}

func (k *K8sCon) Close() error {
	if k.fifoFile != nil {
		_ = k.fifoFile.Close()
	}
	if k.fifoPath != "" {
		_ = os.Remove(k.fifoPath)
	}
	_ = k.LocalCommand.Close()
	return k.proxy.Close()
}

// createAliasFifo creates a named pipe outside the tmpfs sandbox the
// generated shell will mount, and opens it read-write (non-blocking open,
// so we never wait on the sandboxed shell to connect first). The sandboxed
// `pam-alias` shell function writes "name=command" lines into it; we read
// them back here, in Koko's own process, and persist them via the API -
// the sandboxed shell itself never gets network access or API credentials.
func createAliasFifo() (string, *os.File, error) {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("jms_k8s_alias_%s.fifo", common.UUID()))
	if err := syscall.Mkfifo(path, 0600); err != nil {
		return "", nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR, os.ModeNamedPipe)
	if err != nil {
		_ = os.Remove(path)
		return "", nil, err
	}
	return path, f, nil
}

func syncPendingK8sAliases(fifoFile *os.File, jmsService *service.JMService, tokenID string) {
	scanner := bufio.NewScanner(fifoFile)
	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		name, command := line[:idx], line[idx+1:]
		if err := jmsService.SetK8sShellAlias(tokenID, name, command); err != nil {
			logger.Errorf("K8sCon save alias %q err: %s", name, err)
		}
	}
}

// watchK8sSandboxIsolation waits for the sandboxed shell process to exit and
// logs a distinct, loud error if it exited via one of the isolation-check
// sentinel exit codes from init-kubectl.sh (the mount namespace/tmpfs
// sandbox failed to actually isolate the session).
func watchK8sSandboxIsolation(lcmd *localcommand.LocalCommand) {
	_ = lcmd.Wait()
	switch lcmd.ExitCode() {
	case 97, 98:
		logger.Errorf(
			"K8sCon SANDBOX ISOLATION FAILED (exit code %d): the mount namespace/tmpfs isolation "+
				"for this Kubernetes shell session did not take effect - refusing to treat this as a "+
				"normal disconnect. Check that the koko container/pod has CAP_SYS_ADMIN.",
			lcmd.ExitCode(),
		)
	}
}

var osUsernameInvalidCharsRe = regexp.MustCompile(`[^a-z0-9_-]`)

// SanitizeK8sOSUsername turns an arbitrary JumpServer username into a safe,
// short, collision-resistant Linux username for the sandboxed kubectl shell,
// so `whoami`/`id`/$PS1 reflect the real JumpServer user instead of a single
// shared `jms_k8s_user` account for everyone.
func SanitizeK8sOSUsername(rawUsername, userID string) string {
	s := strings.ToLower(rawUsername)
	s = osUsernameInvalidCharsRe.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_-")
	if s == "" {
		s = "user"
	}
	const maxBase = 24
	if len(s) > maxBase {
		s = s[:maxBase]
	}
	suffix := ""
	if userID != "" {
		sum := sha1.Sum([]byte(userID))
		suffix = "_" + hex.EncodeToString(sum[:])[:6]
	}
	return "jms_" + s + suffix
}

type k8sOptions struct {
	ClusterServer string // https://172.16.10.51:8443
	Username      string // user's system username
	Token         string // authorization token
	IsSkipTls     bool
	ExtraEnv      map[string]string
	DEBUG         bool

	RealUsername    string   // sanitized real JumpServer username (OS-safe), for the sandbox's OS identity
	RealUserDisplay string   // real JumpServer username, for display only (PS1)
	AliasLines      []string // pre-formatted, shell-ready `alias name='command'` lines
	TokenID         string   // connect token id, used to persist new aliases via the API
	JMService       *service.JMService
	AliasFifoPath   string // set internally once the alias-sync fifo is created

	win Windows
}

func (o *k8sOptions) K8sCfg() *rest.Config {
	kubeConf := &rest.Config{
		Host:        o.ClusterServer,
		BearerToken: o.Token,
	}
	if o.IsSkipTls {
		kubeConf.Insecure = true
	}
	return kubeConf
}

func (o *k8sOptions) Env() []string {
	token, err := utils.Encrypt(o.Token, config.CipherKey)
	if err != nil {
		logger.Errorf("Encrypt k8s token err: %s", err)
		token = o.Token
	}
	skipTls := "true"
	if !o.IsSkipTls {
		skipTls = "false"
	}
	k8sName := strings.Trim(strconv.Quote(o.ExtraEnv["K8sName"]), "\"")
	k8sName = strings.ReplaceAll(k8sName, "`", "\\`")
	return []string{
		fmt.Sprintf("KUBECTL_USER=%s", o.Username),
		fmt.Sprintf("KUBECTL_CLUSTER=%s", o.ClusterServer),
		fmt.Sprintf("KUBECTL_INSECURE_SKIP_TLS_VERIFY=%s", skipTls),
		fmt.Sprintf("K8S_ENCRYPTED_TOKEN=%s", token),
		fmt.Sprintf("WELCOME_BANNER=%s", config.KubectlBanner),
		fmt.Sprintf("K8S_NAME=%s", k8sName),
	}
}

func startK8SLocalCommand(env []string) (*localcommand.LocalCommand, error) {
	pwd, _ := os.Getwd()
	shPath := filepath.Join(pwd, k8sInitFilename)
	argv := []string{
		"--fork",
		"--pid",
		"--mount-proc",
		shPath,
	}
	// Record our own mount namespace before spawning `unshare`, so
	// init-kubectl.sh can verify it actually landed in a *different* mount
	// namespace instead of silently continuing in a shared one.
	if parentNs, err := os.Readlink("/proc/self/ns/mnt"); err == nil {
		env = append(env, fmt.Sprintf("JMS_PARENT_MNT_NS=%s", parentNs))
	} else {
		logger.Errorf("K8sCon read /proc/self/ns/mnt err: %s", err)
	}
	return localcommand.New("unshare", argv, localcommand.WithEnv(env))
}

type K8sOption func(*k8sOptions)

func K8sUsername(username string) K8sOption {
	return func(args *k8sOptions) {
		args.Username = username
	}
}

func K8sToken(token string) K8sOption {
	return func(args *k8sOptions) {
		args.Token = token
	}
}

func K8sClusterServer(clusterServer string) K8sOption {
	return func(args *k8sOptions) {
		args.ClusterServer = clusterServer
	}
}

func K8sExtraEnvs(envs map[string]string) K8sOption {
	return func(args *k8sOptions) {
		args.ExtraEnv = envs
	}
}

func K8sSkipTls(isSkipTls bool) K8sOption {
	return func(args *k8sOptions) {
		args.IsSkipTls = isSkipTls
	}
}

func K8sPtyWin(win Windows) K8sOption {
	return func(args *k8sOptions) {
		args.win = win
	}
}

func K8sDebug(debug bool) K8sOption {
	return func(args *k8sOptions) {
		args.DEBUG = debug
	}
}

// K8sRealUser sets the real, logged-in JumpServer user's identity for this
// session - sanitizedUsername must already be a safe Linux username (see
// SanitizeK8sOSUsername), displayUsername is only used for the shell prompt.
func K8sRealUser(sanitizedUsername, displayUsername string) K8sOption {
	return func(args *k8sOptions) {
		args.RealUsername = sanitizedUsername
		args.RealUserDisplay = displayUsername
	}
}

// K8sAliasLines sets the real user's saved kubectl aliases, already
// formatted as ready-to-source shell lines (e.g. `alias kgpa='kubectl get pods -A'`).
func K8sAliasLines(lines []string) K8sOption {
	return func(args *k8sOptions) {
		args.AliasLines = lines
	}
}

// K8sTokenID sets the connect token id used to persist newly-saved aliases.
func K8sTokenID(tokenID string) K8sOption {
	return func(args *k8sOptions) {
		args.TokenID = tokenID
	}
}

// K8sJMService sets the JMService client used to fetch/persist the real
// user's kubectl aliases for this session.
func K8sJMService(jmsService *service.JMService) K8sOption {
	return func(args *k8sOptions) {
		args.JMService = jmsService
	}
}
