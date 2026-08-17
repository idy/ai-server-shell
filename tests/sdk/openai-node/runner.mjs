import process from 'node:process';

import { runHTTPCaseRegistry } from './cases/common.mjs';

const baseURL = process.env.AI_SHELL_BASE_URL;
const manifestPath = process.env.AI_SHELL_OPERATION_MANIFEST;
const specPath = process.env.AI_SHELL_OPENAPI_SPEC;
if (!baseURL || !manifestPath || !specPath) {
  throw new Error('AI_SHELL_BASE_URL, AI_SHELL_OPERATION_MANIFEST, and AI_SHELL_OPENAPI_SPEC are required');
}

const report = await runHTTPCaseRegistry({
  baseURL,
  apiKey: process.env.AI_SHELL_API_KEY || 'integration-test-key',
  manifestPath,
  specPath,
});
process.stdout.write(`${JSON.stringify(report)}\n`);
