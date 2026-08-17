import fs from 'node:fs/promises';

import OpenAI from 'openai';
import { ResponsesWS } from 'openai/resources/responses/ws';

import { assertObservedEvent, eventFixture, waitForEvent, waitNodeSocketOpen } from './websocket-common.mjs';

export async function runResponsesWebSocketCases({ baseURL, apiKey, inventoryPath }) {
  const inventory = JSON.parse(await fs.readFile(inventoryPath, 'utf8'));
  const client = new OpenAI({ apiKey, baseURL, maxRetries: 0, timeout: 10_000 });
  const responses = new ResponsesWS(client);
  await waitNodeSocketOpen(responses.socket);
  const covered = [];
  const surface = 'responses_websocket';
  try {
    for (const type of inventory.surfaces[surface].client) {
      responses.send(eventFixture(type));
      covered.push(`${surface}/client/${type}`);
    }
    for (const type of inventory.surfaces[surface].server) {
      const expected = eventFixture(type);
      const observed = waitForEvent(responses, type);
      responses.send({
        type: 'response.create', model: 'integration-model', input: 'fixture', stream: true,
        fixture_server_type: type,
        fixture_server_payload: expected,
      });
      const event = await observed;
      assertObservedEvent(type, expected, event);
      covered.push(`${surface}/server/${type}`);
    }
  } finally {
    responses.close();
  }
  return covered;
}
