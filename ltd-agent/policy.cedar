// Cedar Authorization Policy for ltd-agent
// Principal: Agent::"Local"
// Action:    Action::"Execute"
// Resource:  Command::"CLI"
// Context:   { executable: String, full_command: String }

// 1. Permit Policy:
// Allows Action::"Execute" if context.executable is in ["pytest", "npm", "mvn", "git", "ls"]
permit (
    principal == Agent::"Local",
    action == Action::"Execute",
    resource == Command::"CLI"
) when {
    ["pytest", "npm", "mvn", "git", "ls"].contains(context.executable)
};

// 2. Forbid Policy:
// Strict forbid overriding permit if context.full_command contains any forbidden substrings:
// "curl", "wget", "nc", "ssh", "~/.aws", ".env"
forbid (
    principal == Agent::"Local",
    action == Action::"Execute",
    resource == Command::"CLI"
) when {
    context.full_command like "*curl*" ||
    context.full_command like "*wget*" ||
    context.full_command like "*nc*" ||
    context.full_command like "*ssh*" ||
    context.full_command like "*~/.aws*" ||
    context.full_command like "*.env*"
};
