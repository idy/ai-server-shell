import process from 'node:process';

import { buildBackendFixtures } from './cases/common.mjs';

const manifestPath = process.env.AI_SHELL_OPERATION_MANIFEST;
const specPath = process.env.AI_SHELL_OPENAPI_SPEC;
if (!manifestPath || !specPath) throw new Error('AI_SHELL_OPERATION_MANIFEST and AI_SHELL_OPENAPI_SPEC are required');
const fixtures = await buildBackendFixtures({
  baseURL: 'http://fixture.invalid/v1', apiKey: 'fixture-key', manifestPath, specPath,
});
process.stdout.write(`${JSON.stringify(fixtures)}\n`);
