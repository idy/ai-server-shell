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

Never enable `full` without a disposable project and explicit cost/mutation
approval. The current M1 implementation deliberately reports the remaining
full-profile surface as unverified rather than silently spending or mutating.
