import OpenAI, { toFile } from 'openai';

const baseURL = process.env.AI_SHELL_BASE_URL;
const apiKey = process.env.AI_SHELL_API_KEY;
const profile = process.env.OPENAI_COMPAT_PROFILE;
if (!baseURL || !apiKey || !['safe', 'paid', 'mutation'].includes(profile)) {
  throw new Error('AI_SHELL_BASE_URL, AI_SHELL_API_KEY, and a valid OPENAI_COMPAT_PROFILE are required');
}

const client = new OpenAI({ apiKey, baseURL, maxRetries: 0, timeout: 30_000 });
const cases = [];
let failureReported = false;
const reportFailure = (error) => {
  if (failureReported) return;
  failureReported = true;
  const names = { safe: 'models.list', paid: 'embeddings.create', mutation: 'files.create-delete' };
  const result = cases.at(-1) ?? {
    name: names[profile], observation: {},
    cleanup: { required: profile === 'mutation', outcome: profile === 'mutation' ? 'not_started' : 'not_required' },
  };
  result.outcome = 'FAIL';
  if (result.cleanup.required && result.cleanup.outcome !== 'verified') result.cleanup.outcome = 'failed';
  result.reason = String(error?.message ?? error).replaceAll(apiKey, '[REDACTED]').slice(0, 1000);
  process.stdout.write(`${JSON.stringify({ profile, target: process.env.OPENAI_COMPAT_TARGET, cases: [result] })}\n`);
  process.exitCode = 1;
};
process.once('uncaughtException', reportFailure);
process.once('unhandledRejection', reportFailure);

if (profile === 'safe') {
  const page = await client.models.list();
  cases.push({
    name: 'models.list', outcome: 'PASS', cleanup: { required: false, outcome: 'not_required' },
    observation: { object: page.object, data: page.data.map((model) => ({ id: model.id, object: model.object })).sort((a, b) => a.id.localeCompare(b.id)) },
  });
}

if (profile === 'paid') {
  const result = await client.embeddings.create({
    model: process.env.OPENAI_COMPAT_EMBEDDING_MODEL || 'text-embedding-3-small',
    input: 'ai-server-shell compatibility probe',
  });
  cases.push({
    name: 'embeddings.create', outcome: 'PASS', cleanup: { required: false, outcome: 'not_required' },
    observation: {
      object: result.object, model_present: typeof result.model === 'string',
      vector_count: result.data.length, vector_nonempty: result.data.every((item) => item.embedding.length > 0),
      usage_present: Number.isInteger(result.usage?.total_tokens),
    },
  });
}

if (profile === 'mutation') {
  let file;
  let cleanup = { required: true, outcome: 'not_started' };
  try {
    file = await client.files.create({
      file: await toFile(Buffer.from('{"custom_id":"compat","method":"POST","url":"/v1/responses","body":{"model":"gpt-4.1-mini","input":"test"}}\n'), 'compatibility.jsonl'),
      purpose: 'batch',
    });
    cases.push({
      name: 'files.create-delete', outcome: 'PASS',
      observation: { object: file.object, filename: file.filename, purpose: file.purpose, id_present: typeof file.id === 'string' },
      cleanup,
    });
  } finally {
    if (file?.id) {
      const deleted = await client.files.delete(file.id);
      cleanup = { required: true, outcome: deleted.deleted ? 'verified' : 'failed' };
      cases.at(-1).cleanup = cleanup;
      if (!deleted.deleted) throw new Error(`cleanup was not verified for ${file.id}`);
    }
  }
}

process.stdout.write(`${JSON.stringify({ profile, target: process.env.OPENAI_COMPAT_TARGET, cases })}\n`);
