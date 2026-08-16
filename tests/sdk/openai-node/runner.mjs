import fs from 'node:fs/promises';
import process from 'node:process';

import OpenAI from 'openai';

const baseURL = process.env.AI_SHELL_BASE_URL;
const manifestPath = process.env.AI_SHELL_OPERATION_MANIFEST;
if (!baseURL || !manifestPath) {
  throw new Error('AI_SHELL_BASE_URL and AI_SHELL_OPERATION_MANIFEST are required');
}

const operations = JSON.parse(await fs.readFile(manifestPath, 'utf8'));
const client = new OpenAI({
  apiKey: process.env.AI_SHELL_API_KEY || 'integration-test-key',
  baseURL,
  maxRetries: 0,
  timeout: 10_000,
});

const failures = [];
for (const operation of operations) {
  const method = operation.Method.toLowerCase();
  const path = operation.Path.replaceAll(/\{[^}]+\}/g, 'test-id');
  try {
    const options = ['post', 'put', 'patch'].includes(method) ? { body: {} } : {};
    await client[method](path, options);
  } catch (error) {
    failures.push({
      id: operation.OperationID,
      method: operation.Method,
      path,
      message: String(error?.message ?? error).slice(0, 500),
    });
  }
}

// Exercise a generated resource helper in addition to the exhaustive raw SDK
// transport inventory. The fake backend returns the canonical empty model list.
try {
  const page = await client.models.list();
  if (!Array.isArray(page.data)) {
    throw new Error('models.list() did not return a list page');
  }
} catch (error) {
  failures.push({ id: 'sdk.models.list', message: String(error?.message ?? error).slice(0, 500) });
}

process.stdout.write(`${JSON.stringify({
  sdk: 'openai-node',
  version: '7.4.0',
  expected: operations.length + 1,
  passed: operations.length + 1 - failures.length,
  failures,
})}\n`);
if (failures.length !== 0) {
  process.exitCode = 1;
}
