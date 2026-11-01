                            return Ok(None);
                        }
                    }
                }

                if !needs_follow_up {
                    last_agent_message = sampling_request_last_agent_message;
                    let stop_outcome = run_turn_stop_hooks(
                        &sess,
                        &step_context,
                        stop_hook_active,
                        last_agent_message.clone(),
                    )
                    .await;
                    if matches!(
                        turn_context.session_source,
                        SessionSource::Internal(InternalSessionSource::MemoryConsolidation)
                    ) && (stop_outcome.should_block || stop_outcome.should_stop)
                    {
                        // Do not feed managed rejections back into an unattended memory loop.
                        return Err(CodexErr::InvalidRequest(
                            "Memory consolidation was rejected by a Stop hook.".to_string(),
                        ));
                    }
                    if stop_outcome.should_block {
                        if let Some(hook_prompt_message) =
                            build_hook_prompt_message(&stop_outcome.continuation_fragments)
                        {
                            sess.record_response_item_and_emit_turn_item(
                                &turn_context,
                                &step_context.settings.model_info,
                                hook_prompt_message,
                            )
                            .await;
                            sess.input_queue
                                .accept_mailbox_delivery_for_current_turn(
                                    &sess.active_turn,
                                    &turn_context.sub_id,
                                )
                                .await;
                            stop_hook_active = true;
                            continue;
                        } else {
                            sess.send_event(
                                &turn_context,
                                EventMsg::Warning(WarningEvent {
                                    message: "Stop hook requested continuation without a prompt; ignoring the block.".to_string(),
                                }),
                            )
                            .await;
                        }
                    }
                    if stop_outcome.should_stop {
                        break;
                    }
                    if run_legacy_after_agent_hook(
