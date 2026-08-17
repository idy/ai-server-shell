# OpenAI live compatibility

This opt-in suite compares the same official `openai` Node SDK call against
OpenAI directly and against a local Shell whose test-only backend calls OpenAI.
The incoming Shell credential is deliberately different from `OPENAI_API_KEY`.

The `safe` profile currently performs the read-only, no-inference `models.list`
case. It creates no resources, so cleanup is `not-required`. A missing key,
network error, upstream denial, adapter error, or comparison failure fails the
test; none is converted to a compatibility pass.

```sh
npm --prefix tests/sdk/openai-node ci
OPENAI_COMPAT_PROFILE=safe go test -tags=compatibility -v ./tests/compatibility/openai
```

The bounded paid profile creates no persistent resource:

```sh
OPENAI_COMPAT_PROFILE=paid OPENAI_COMPAT_ALLOW_COST=1 \
  go test -tags=compatibility -v ./tests/compatibility/openai
```

The mutation profile uploads one batch-purpose JSONL file and verifies its
deletion. Use only a disposable project:

```sh
OPENAI_COMPAT_PROFILE=mutation OPENAI_COMPAT_ALLOW_COST=1 \
  OPENAI_COMPAT_ALLOW_MUTATION=1 \
  go test -tags=compatibility -v ./tests/compatibility/openai
```

Missing credentials, transport/adapter failures, assertion failures, and failed
cleanup are `FAIL`. `SKIP` is reserved for an explicitly verified upstream
permission, model, account, or regional restriction and must retain its
sanitized reason; current named cases do not synthesize `SKIP` outcomes.
