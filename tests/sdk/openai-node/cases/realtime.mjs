import fs from 'node:fs/promises';

import OpenAI from 'openai';
import { OpenAIRealtimeWebSocket } from 'openai/realtime/websocket';

import { assertObservedEvent, eventFixture, waitForEvent, waitDOMSocketOpen } from './websocket-common.mjs';

export async function runRealtimeCases({ baseURL, apiKey, inventoryPath }) {
  const inventory = JSON.parse(await fs.readFile(inventoryPath, 'utf8'));
  const client = new OpenAI({ apiKey, baseURL, maxRetries: 0, timeout: 10_000 });
  const realtime = new OpenAIRealtimeWebSocket({ model: 'integration-model' }, client);
  await waitDOMSocketOpen(realtime.socket);
  const covered = [];
  try {
    for (const surface of ['realtime', 'realtime_translation']) {
      for (const type of inventory.surfaces[surface].client) {
        realtime.send(eventFixture(type));
        covered.push(`${surface}/client/${type}`);
      }
      for (const type of inventory.surfaces[surface].server) {
        const expected = eventFixture(type);
        const observed = waitForEvent(realtime, type);
        realtime.send({
          type: 'session.update',
          session: { type: 'realtime', output_modalities: ['text'] },
          fixture_server_type: type,
          fixture_server_payload: expected,
        });
        const event = await observed;
        assertObservedEvent(type, expected, event);
        covered.push(`${surface}/server/${type}`);
      }
    }
  } finally {
    realtime.close();
  }
  return covered;
}
