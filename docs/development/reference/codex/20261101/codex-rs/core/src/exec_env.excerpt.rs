pub use codex_apply_patch::CODEX_APPLY_PATCH_PRESERVE_LINE_ENDINGS_ENV_VAR;
use codex_protocol::SessionId;
use codex_protocol::ThreadId;
#[cfg(test)]
use codex_protocol::config_types::EnvironmentVariablePattern;
use codex_protocol::config_types::ShellEnvironmentPolicy;
use codex_protocol::models::ActivePermissionProfile;
use codex_protocol::shell_environment;
use std::collections::HashMap;

pub use codex_protocol::shell_environment::CODEX_SESSION_ID_ENV_VAR;
pub use codex_protocol::shell_environment::CODEX_THREAD_ID_ENV_VAR;
pub use codex_protocol::shell_environment::CODEX_TOOL_CALL_ID_ENV_VAR;
pub(crate) use codex_protocol::shell_environment::set_tool_call_id_env_var;

pub(crate) const CODEX_VERSION_ENV_VAR: &str = "CODEX_VERSION";

/// Informational name of the active permission profile. Child processes can
/// overwrite this value, so it must not be treated as proof of enforcement.
pub const CODEX_PERMISSION_PROFILE_ENV_VAR: &str = "CODEX_PERMISSION_PROFILE";

/// Construct an environment map based on the rules in the specified policy. The
/// resulting map can be passed directly to `Command::envs()` after calling
/// `env_clear()` to ensure no unintended variables are leaked to the spawned
/// process.
///
/// The derivation follows the algorithm documented in the struct-level comment
/// for [`ShellEnvironmentPolicy`].
///
/// `CODEX_THREAD_ID` is injected when a thread id is provided, even when
/// `include_only` is set.
pub fn create_env(
    policy: &ShellEnvironmentPolicy,
    thread_id: Option<ThreadId>,
) -> HashMap<String, String> {
    let thread_id = thread_id.map(|thread_id| thread_id.to_string());
    shell_environment::create_env(policy, thread_id.as_deref())
}

/// Exposes the shared root-session identity and harness version to shell commands.
pub(crate) fn inject_session_env(env: &mut HashMap<String, String>, session_id: SessionId) {
    env.insert(CODEX_SESSION_ID_ENV_VAR.to_string(), session_id.to_string());
    if cfg!(windows) {
        env.retain(|key, _| !key.eq_ignore_ascii_case(CODEX_VERSION_ENV_VAR));
    }
    env.insert(
        CODEX_VERSION_ENV_VAR.to_string(),
