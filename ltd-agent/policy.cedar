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
        "ls", "dir", "mkdir", "echo", "pwd", "cat"
    ].contains(context.executable)
};

// 2. Forbid Policy:
// Strict forbid overriding permit if context.full_command contains any forbidden substrings:
// Credentials, sensitive config files, and unauthorized network egress utilities.
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
    context.full_command like "*~/.aws*" ||
    context.full_command like "*.env*" ||
    context.full_command like "*.ssh*"
};

