import OpenAI, { toFile } from 'openai';

const baseURL = process.env.AI_SHELL_BASE_URL;
if (!baseURL) throw new Error('AI_SHELL_BASE_URL is required');
const client = new OpenAI({
  apiKey: process.env.AI_SHELL_API_KEY || 'integration-test-key',
  baseURL,
  maxRetries: 0,
  timeout: 10_000,
});

await client.models.list({ limit: 1 });
await client.embeddings.create({ model: 'text-embedding-test', input: 'hello' });
await client.files.create({
  file: await toFile(Buffer.from('{"test":true}\n'), 'input.jsonl'),
  purpose: 'batch',
});
const content = await client.files.content('file_test');
if (await content.text() !== 'binary-test') throw new Error('unexpected binary file content');

process.stdout.write(`${JSON.stringify({ passed: 4 })}\n`);
