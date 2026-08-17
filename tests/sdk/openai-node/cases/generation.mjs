export const generationCapabilities = new Set(['responses', 'chat', 'completions', 'conversations', 'embeddings']);

export async function runGenerationScenarios(client) {
  const stream = await client.responses.create(
    { model: 'gpt-test', input: 'hello', stream: true },
    { headers: { 'X-AI-Shell-Stream-Case': 'responses.basic' } },
  );
  const observed = [];
  for await (const event of stream) observed.push(event.type);
  const expected = ['response.created', 'response.output_text.delta', 'response.completed'];
  if (JSON.stringify(observed) !== JSON.stringify(expected)) {
    throw new Error(`Responses SSE order mismatch: ${JSON.stringify(observed)}`);
  }
  return ['responses.basic'];
}
