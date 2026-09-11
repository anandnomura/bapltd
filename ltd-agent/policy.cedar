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
        "ls", "dir", "mkdir", "echo", "pwd", "cat", "type", "printenv",
        "cmd", "powershell", "pwsh",
        "grep", "findstr", "head", "tail", "wc", "sort", "uniq", "more"
    ].contains(context.executable)
};

// 2. Forbid Policy:
// Strict forbid overriding permit for:
// - Exfiltration utilities: curl, wget, nc, ssh, socat, PowerShell web cmdlets
// - Sensitive credential directories: ~/.aws, .ssh
// - Dumping secret content from environment/credential files (e.g. cat .env, type .env)
forbid (
    principal == Agent::"Local",
    action == Action::"Execute",
    resource == Command::"CLI"
) when {
    context.full_command like "*curl*" ||
    context.full_command like "*wget*" ||
    context.full_command like "*nc*" ||
    context.full_command like "*ssh*" ||
    context.full_command like "*socat*" ||
    context.full_command like "*Invoke-WebRequest*" ||
    context.full_command like "*Invoke-RestMethod*" ||
    context.full_command like "*iwr *" ||
    context.full_command like "*irm *" ||
    context.full_command like "*DownloadString*" ||
    context.full_command like "*DownloadFile*" ||
    context.full_command like "*~/.aws*" ||
    context.full_command like "*.ssh*" ||
    context.full_command like "*cat*.env*" ||
    context.full_command like "*type*.env*" ||
    context.full_command like "*head*.env*" ||
    context.full_command like "*tail*.env*" ||
    context.full_command like "*grep*.env*" ||
    context.full_command like "*findstr*.env*" ||
    context.full_command like "*more*.env*" ||
    context.full_command like "*Get-Content*.env*" ||
    context.full_command like "*gc *.env*" ||
    context.full_command like "*<*.env*"
};


