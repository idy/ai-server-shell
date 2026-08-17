const timeout = (label) => new Promise((_, reject) => {
  const timer = setTimeout(() => reject(new Error(`${label} timed out`)), 5_000);
  timer.unref();
});

export async function waitDOMSocketOpen(socket) {
  if (socket.readyState === 1) return;
  await Promise.race([
    new Promise((resolve, reject) => {
      socket.addEventListener('open', resolve, { once: true });
      socket.addEventListener('error', reject, { once: true });
    }),
    timeout('Realtime WebSocket open'),
  ]);
}

export async function waitNodeSocketOpen(socket) {
  if (socket.readyState === 1) return;
  await Promise.race([
    new Promise((resolve, reject) => {
      socket.once('open', resolve);
      socket.once('error', reject);
    }),
    timeout('Responses WebSocket open'),
  ]);
}

export function waitForEvent(emitter, type) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      cleanup();
      reject(new Error(`${type} timed out`));
    }, 5_000);
    timer.unref();
    const onEvent = (event) => {
      cleanup();
      resolve(event);
    };
    const onError = (error) => {
      cleanup();
      reject(error);
    };
    const cleanup = () => {
      clearTimeout(timer);
      emitter.off(type, onEvent);
      if (type !== 'error') emitter.off('error', onError);
    };
    emitter.once(type, onEvent);
    if (type !== 'error') emitter.once('error', onError);
  });
}

export function assertObservedEvent(type, expected, observed) {
  if (type === 'error') {
    if (!observed || (typeof observed !== 'object' && !(observed instanceof Error))) {
      throw new Error(`${type}: SDK did not expose its parsed error`);
    }
    return;
  }
  if (observed.type !== type || observed.fixture_marker !== expected.fixture_marker) {
    throw new Error(`${type}: SDK event payload mismatch`);
  }
}

export function eventFixture(type) {
  return {
    type,
    event_id: `event_${type.replaceAll('.', '_')}`,
    sequence_number: 1,
    fixture_marker: `fixture:${type}`,
    audio: 'AA==', delta: 'test', transcript: 'test', text: 'test',
    item_id: 'item_test', response_id: 'resp_test', output_index: 0, content_index: 0,
    part: { type: 'output_text', text: 'test' },
    item: { id: 'item_test', type: 'message', role: 'assistant', content: [] },
    response: { id: 'resp_test', object: 'response', status: 'completed', output: [], created_at: 1, error: null, incomplete_details: null },
    session: { id: 'sess_test', object: 'realtime.session', type: 'realtime' },
    conversation: { id: 'conv_test', object: 'realtime.conversation' },
    rate_limits: [],
    error: { type: 'invalid_request_error', code: 'fixture_error', message: 'fixture' },
  };
}
