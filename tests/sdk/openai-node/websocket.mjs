import process from 'node:process';

import OpenAI from 'openai';
import { OpenAIRealtimeWebSocket } from 'openai/realtime/websocket';
import { ResponsesWS } from 'openai/resources/responses/ws';

const baseURL = process.env.AI_SHELL_BASE_URL;
if (!baseURL) {
  throw new Error('AI_SHELL_BASE_URL is required');
}
const client = new OpenAI({
  apiKey: process.env.AI_SHELL_API_KEY || 'integration-test-key',
  baseURL,
  maxRetries: 0,
  timeout: 10_000,
});

const timeout = (label) => new Promise((_, reject) => {
  const timer = setTimeout(() => reject(new Error(`${label} timed out`)), 5_000);
  timer.unref();
});

async function waitDOMSocketOpen(socket) {
  if (socket.readyState === 1) return;
  await Promise.race([
    new Promise((resolve, reject) => {
      socket.addEventListener('open', resolve, { once: true });
      socket.addEventListener('error', reject, { once: true });
    }),
    timeout('Realtime WebSocket open'),
  ]);
}

async function waitNodeSocketOpen(socket) {
  if (socket.readyState === 1) return;
  await Promise.race([
    new Promise((resolve, reject) => {
      socket.once('open', resolve);
      socket.once('error', reject);
    }),
    timeout('Responses WebSocket open'),
  ]);
}

async function runRealtime() {
  const realtime = new OpenAIRealtimeWebSocket({ model: 'integration-model' }, client);
  await waitDOMSocketOpen(realtime.socket);
  const updated = Promise.race([
    new Promise((resolve, reject) => {
      realtime.on('session.updated', resolve);
      realtime.on('error', reject);
    }),
    timeout('session.updated'),
  ]);
  realtime.send({ type: 'session.update', session: { type: 'realtime', output_modalities: ['text'] } });
  const event = await updated;
  realtime.close();
  if (event.type !== 'session.updated') throw new Error('unexpected Realtime event');
  return event.type;
}

async function runResponses() {
  const responses = new ResponsesWS(client);
  await waitNodeSocketOpen(responses.socket);
  const completed = Promise.race([
    new Promise((resolve, reject) => {
      responses.on('response.completed', resolve);
      responses.on('error', reject);
    }),
    timeout('response.completed'),
  ]);
  responses.send({ type: 'response.create', model: 'integration-model', input: 'hello', stream: true });
  const event = await completed;
  responses.close();
  if (event.type !== 'response.completed') throw new Error('unexpected Responses event');
  return event.type;
}

const realtime = await runRealtime();
const responses = await runResponses();
process.stdout.write(`${JSON.stringify({ realtime, responses })}\n`);
