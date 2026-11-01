
    /// Only synchronous hooks can apply control effects.
    pub(crate) fn can_apply_control_effects(&self) -> bool {
        self.execution_mode() == HookExecutionMode::Sync
    }
