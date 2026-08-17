import process from 'node:process';

import { runRealtimeCases } from './cases/realtime.mjs';
import { runResponsesWebSocketCases } from './cases/responses-websocket.mjs';

const baseURL = process.env.AI_SHELL_BASE_URL;
const inventoryPath = process.env.AI_SHELL_EVENT_INVENTORY;
if (!baseURL || !inventoryPath) throw new Error('AI_SHELL_BASE_URL and AI_SHELL_EVENT_INVENTORY are required');
const settings = { baseURL, inventoryPath, apiKey: process.env.AI_SHELL_API_KEY || 'integration-test-key' };
const realtime = await runRealtimeCases(settings);
const responses = await runResponsesWebSocketCases(settings);
const covered = [...realtime, ...responses];
process.stdout.write(`${JSON.stringify({ expected: 121, passed: covered.length, covered })}\n`);
