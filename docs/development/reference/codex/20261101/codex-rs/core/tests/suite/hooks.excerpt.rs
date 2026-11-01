#[tokio::test]
async fn stop_hook_can_block_multiple_times_in_same_turn() -> Result<()> {
    skip_if_no_network!(Ok(()));

    let server = start_mock_server().await;
    let responses = mount_sse_sequence(
        &server,
        vec![
            sse(vec![
                ev_response_created("resp-1"),
                ev_assistant_message("msg-1", "draft one"),
                ev_completed("resp-1"),
            ]),
            sse(vec![
                ev_response_created("resp-2"),
                ev_assistant_message("msg-2", "draft two"),
                ev_completed("resp-2"),
            ]),
            sse(vec![
                ev_response_created("resp-3"),
                ev_assistant_message("msg-3", "final draft"),
                ev_completed("resp-3"),
            ]),
        ],
    )
    .await;

    let mut builder = test_codex()
        .with_pre_build_hook(|home| {
            write_stop_hook(
                home,
                &[FIRST_CONTINUATION_PROMPT, SECOND_CONTINUATION_PROMPT],
            )
            .expect("failed to write stop hook test fixture");
        })
        .with_config(trust_discovered_hooks);
    let test = builder.build(&server).await?;

    test.submit_turn("hello from the sea").await?;

    let requests = responses.requests();
    assert_eq!(requests.len(), 3);
    assert_eq!(
        request_hook_prompt_texts(&requests[1]),
        vec![FIRST_CONTINUATION_PROMPT.to_string()],
        "second request should include the first continuation prompt as user hook context",
    );
    assert_eq!(
        request_hook_prompt_texts(&requests[2]),
        vec![
            FIRST_CONTINUATION_PROMPT.to_string(),
            SECOND_CONTINUATION_PROMPT.to_string(),
        ],
        "third request should retain hook prompts in user history",
    );

    let hook_inputs = read_stop_hook_inputs(test.codex_home_path())?;
    assert_eq!(hook_inputs.len(), 3);
    let stop_turn_ids = hook_inputs
        .iter()
        .map(|input| {
            input["turn_id"]
                .as_str()
                .expect("stop hook input turn_id")
                .to_string()
        })
        .collect::<Vec<_>>();
    assert!(
        stop_turn_ids.iter().all(|turn_id| !turn_id.is_empty()),
        "stop hook turn ids should be non-empty",
    );
    let first_stop_turn_id = stop_turn_ids
        .first()
        .expect("stop hook inputs should include a first turn id")
        .clone();
    assert_eq!(
        stop_turn_ids,
        vec![
            first_stop_turn_id.clone(),
            first_stop_turn_id.clone(),
            first_stop_turn_id,
        ],
    );
    assert_eq!(
        hook_inputs
            .iter()
            .map(|input| input["stop_hook_active"]
                .as_bool()
                .expect("stop_hook_active bool"))
            .collect::<Vec<_>>(),
        vec![false, true, true],
    );

    let rollout_path = test.codex.rollout_path().expect("rollout path");
    let rollout_text = fs::read_to_string(&rollout_path)?;
    let hook_prompt_texts = rollout_hook_prompt_texts(&rollout_text)?;
    assert!(
        hook_prompt_texts.contains(&FIRST_CONTINUATION_PROMPT.to_string()),
        "rollout should persist the first continuation prompt",
    );
    assert!(
        hook_prompt_texts.contains(&SECOND_CONTINUATION_PROMPT.to_string()),
        "rollout should persist the second continuation prompt",
    );
