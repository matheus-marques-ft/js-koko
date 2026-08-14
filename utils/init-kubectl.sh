#!/bin/bash
set -e

# Real JumpServer user identity for this session (sanitized OS-safe username,
# already computed by Koko - see srvconn.SanitizeK8sOSUsername). Falls back
# to the old shared account when not provided (e.g. an older koko build).
K8S_OS_USER="${JMS_REAL_USER:-jms_k8s_user}"

function init_jms_k8s_user(){
    echo `getent passwd | grep "${K8S_OS_USER}" || useradd -M -U -d /nonexistent "${K8S_OS_USER}"` > /dev/null 2>&1
    echo `getent passwd | grep "${K8S_OS_USER}" | grep '/nonexistent'  || usermod -d /nonexistent "${K8S_OS_USER}"` > /dev/null 2>&1
    echo `getent group | grep "${K8S_OS_USER}" || groupadd "${K8S_OS_USER}"` > /dev/null 2>&1
}
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
init_jms_k8s_user

if [ "${WELCOME_BANNER}" ]; then
    echo ${WELCOME_BANNER}
fi

mkdir -p /nonexistent
mount -t tmpfs -o size=10M tmpfs /nonexistent

# --- Sandbox isolation hardening ---
# `set -e` alone is not a reliable guard here (it silently stops applying
# inside pipelines/conditionals, and several lines in this script
# deliberately swallow errors via the `echo \`cmd\`` idiom). Verify the
# isolation actually took effect and refuse to continue otherwise, instead
# of silently handing out a shell that shares state with other sessions.
if [ "$(stat -c %d /)" = "$(stat -c %d /nonexistent)" ]; then
    echo "FATAL: /nonexistent is not on a separate filesystem from / - tmpfs mount failed or sandbox isolation is not in effect, refusing to start session" >&2
    exit 97
fi
if [ -n "${JMS_PARENT_MNT_NS}" ]; then
    current_mnt_ns="$(readlink /proc/self/ns/mnt)"
    if [ "${current_mnt_ns}" = "${JMS_PARENT_MNT_NS}" ]; then
        echo "FATAL: mount namespace was not actually unshared - sandbox isolation is not in effect, refusing to start session" >&2
        exit 98
    fi
fi
# --- End isolation hardening ---

cd /nonexistent
touch .bashrc
echo "PS1=\"${JMS_REAL_USER_DISPLAY:-${K8S_OS_USER}}@${K8S_NAME}# \"" >> .bashrc
echo "export TERM=xterm" >> .bashrc
echo "source /usr/share/bash-completion/bash_completion" >> .bashrc
echo 'source /opt/kubectl-aliases/.kubectl_aliases' >> .bashrc
echo 'source <(kubectl completion bash)' >> .bashrc
echo 'complete -F __start_kubectl k' >> .bashrc

# Real user's own saved aliases (fetched by Koko via the API), appended
# after the static defaults above so they can override them if desired.
if [ -n "${JMS_ALIAS_LINES_B64}" ]; then
    echo "${JMS_ALIAS_LINES_B64}" | base64 -d >> .bashrc
    echo >> .bashrc
fi

# `pam-alias name command` lets the user explicitly persist a kubectl alias
# for their own account, across future sessions. It does NOT call the API
# directly from inside this sandboxed shell (no network/credential access
# here) - it only writes a structured line to a named pipe that lives
# outside this tmpfs; Koko's own trusted process reads it and saves it via
# the API on the other end.
if [ -n "${JMS_ALIAS_FIFO}" ]; then
    cat <<'PAMALIAS' >> .bashrc
pam-alias() {
    if [ -z "$1" ] || [ -z "$2" ]; then
        echo "Usage: pam-alias <name> <command>"
        return 1
    fi
    alias "$1"="$2"
    printf '%s=%s\n' "$1" "$2" >> "${JMS_ALIAS_FIFO}"
    echo "Saved: alias $1='$2' (persists across sessions)"
}
PAMALIAS
fi

mkdir -p .kube

export HOME=/nonexistent
export LANG=en_US.UTF-8

echo `kubectl config set-credentials JumpServer-user --token=${KUBECTL_TOKEN}` > /dev/null 2>&1
echo `kubectl config set-cluster kubernetes --server=${KUBECTL_CLUSTER}` > /dev/null 2>&1
echo `kubectl config set-context kubernetes --namespace=${KUBECTL_NAMESPACE}` > /dev/null 2>&1
echo `kubectl config set-context kubernetes --cluster=kubernetes --user=JumpServer-user` > /dev/null 2>&1
echo `kubectl config use-context kubernetes` > /dev/null 2>&1

if [ ${KUBECTL_INSECURE_SKIP_TLS_VERIFY} == "true" ];then
    {
        clusters=`kubectl config get-clusters | tail -n +2`
        for s in ${clusters[@]}; do
            {
                echo `kubectl config set-cluster ${s} --insecure-skip-tls-verify=true` > /dev/null 2>&1
                echo `kubectl config unset clusters.${s}.certificate-authority-data` > /dev/null 2>&1
            } || {
                echo err > /dev/null 2>&1
            }
        done
    } || {
        echo err > /dev/null 2>&1
    }
fi

chown -R "${K8S_OS_USER}:${K8S_OS_USER}" .kube
chown -R "${K8S_OS_USER}:${K8S_OS_USER}" .bashrc

export TMPDIR=/nonexistent

exec su -s /bin/bash "${K8S_OS_USER}"
