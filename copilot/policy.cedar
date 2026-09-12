// Cedar Authorization Policy for ltd-agent
// Principal: Agent::"Local"
// Action:    Action::"Execute"
// Resource:  Command::"CLI"
// Context:   { executable: String, full_command: String }

// 1. Permit Policy:
// Allows Action::"Execute" for standard developer toolchains, initialization, and build commands:
permit (
    principal == Agent::"Local",
    action == Action::"Execute",
    resource == Command::"CLI"
) when {
    [
        "pytest", "python", "python3", "py", "pip", "pip3",
        "go", "npm", "npx", "yarn", "pnpm", "node",
        "mvn", "gradle", "cargo", "git",
        "java", "javac",
        "ls", "dir", "mkdir", "echo", "pwd", "cat", "type", "printenv",
        "cmd", "powershell", "pwsh",
        "cmd", "powershell", "pwsh", "bash", "sh", "zsh",
        "grep", "findstr", "head", "tail", "wc", "sort", "uniq", "more"
    ].contains(context.executable)
};

// 2. Forbid Policy:
// Strict forbid overriding permit for:
// - Exfiltration utilities: curl, wget, nc, ssh, socat, PowerShell web cmdlets
// - Sensitive credential directories: ~/.aws, .ssh
// - Any tampering, copying, renaming, or dumping of .env credential files
forbid (
    principal == Agent::"Local",
    action == Action::"Execute",
    resource == Command::"CLI"
) when {
    context.full_command like "*curl*" ||
    context.full_command like "*wget*" ||
    context.full_command like "nc *" ||
    context.full_command like "* nc *" ||
    context.full_command like "*|nc*" ||
    context.full_command like "*&nc*" ||
    context.full_command like "*;nc*" ||
    context.full_command like "nc" ||
    context.full_command like "*netcat*" ||
    context.full_command like "*ncat*" ||
    context.full_command like "ssh *" ||
    context.full_command like "* ssh *" ||
    context.full_command like "*|ssh*" ||
    context.full_command like "*&ssh*" ||
    context.full_command like "*;ssh*" ||
    context.full_command like "*socat*" ||
    context.full_command like "*Invoke-WebRequest*" ||
    context.full_command like "*Invoke-RestMethod*" ||
    context.full_command like "*iwr *" ||
    context.full_command like "*irm *" ||
    context.full_command like "*DownloadString*" ||
    context.full_command like "*DownloadFile*" ||
    context.full_command like "*certutil*" ||
    context.full_command like "*bitsadmin*" ||
    context.full_command like "*~/.aws*" ||
    context.full_command like "*.ssh*" ||
    (
        context.full_command like "*.env*" &&
        (
            !(context.executable == "ls" || context.executable == "dir") ||
            context.full_command like "*cat*" ||
            context.full_command like "*type*" ||
            context.full_command like "*more*" ||
            context.full_command like "*head*" ||
            context.full_command like "*tail*" ||
            context.full_command like "*grep*" ||
            context.full_command like "*findstr*" ||
            context.full_command like "*ren*" ||
            context.full_command like "*move*" ||
            context.full_command like "*mv*" ||
            context.full_command like "*cp*" ||
            context.full_command like "*copy*" ||
            context.full_command like "*>*" ||
            context.full_command like "*|*"
        )
    )
};


