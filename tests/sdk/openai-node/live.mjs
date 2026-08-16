import OpenAI from 'openai';

const baseURL = process.env.AI_SHELL_BASE_URL;
const apiKey = process.env.AI_SHELL_API_KEY;
if (!baseURL || !apiKey) throw new Error('AI_SHELL_BASE_URL and AI_SHELL_API_KEY are required');

const client = new OpenAI({ apiKey, baseURL, maxRetries: 0, timeout: 30_000 });
const page = await client.models.list();
const normalized = page.data.map((model) => ({ id: model.id, object: model.object })).sort((a, b) => a.id.localeCompare(b.id));
process.stdout.write(`${JSON.stringify({ operation: 'listModels', object: page.object, data: normalized })}\n`);
