import fs from 'node:fs/promises';

import OpenAI, { toFile } from 'openai';

import { caseFamilyFor } from './http-families.mjs';
import { runGenerationScenarios } from './generation.mjs';

const silenceWAV = await fs.readFile(new URL('../fixtures/one-second-silence.wav', import.meta.url));

export async function runHTTPCaseRegistry({ baseURL, apiKey, manifestPath, specPath }) {
  const plans = await buildHTTPCasePlans({ baseURL, apiKey, manifestPath, specPath });
  const results = [];
  const failures = [];
  for (const plan of plans) {
    const { operation, request, response, helper } = plan;
    try {
      if (!['resource_helper', 'raw_sdk_exception'].includes(operation.SDKCall)) throw new Error('manifest has invalid SDK call ownership');
      const expectedCall = operation.SDKCall === 'raw_sdk_exception' ? 'raw' : 'helper';
      if ((helper ? 'helper' : 'raw') !== expectedCall) throw new Error(`SDK call ownership changed; expected ${expectedCall}`);
      const headers = { 'X-AI-Shell-Case': operation.OperationID };
      const client = makeClient(baseURL, apiKey, headers);
      const started = Date.now();
      const result = helper ? await invokeHelper(client, helper, request) : await invokeRaw(client, operation, request);
      await assertSDKResult(operation, response, result);
      results.push({
        operation: operation.OperationID,
        family: caseFamilyFor(operation.Capability),
        sdk_call: helper ? `${helper.owner.slice('client.'.length)}.${helper.name}` : 'raw',
        request_media: request.mediaType,
        response_media: response.mediaType,
        duration_ms: Date.now() - started,
      });
    } catch (error) {
      failures.push({ operation: operation.OperationID, message: String(error?.message ?? error).slice(0, 1000) });
    }
  }
  const negativeCases = await runNegativeCases(makeClient(baseURL, apiKey));
  const streamCases = await runGenerationScenarios(makeClient(baseURL, apiKey));
  return {
    sdk: 'openai-node', version: '7.4.0', expected: plans.length, passed: results.length,
    helper_cases: results.filter((item) => item.sdk_call !== 'raw').length,
    raw_cases: results.filter((item) => item.sdk_call === 'raw').length,
    negative_cases: negativeCases,
    stream_cases: streamCases,
    failures, results,
  };
}

async function buildHTTPCasePlans({ baseURL, apiKey, manifestPath, specPath }) {
  const [manifest, spec] = await Promise.all([readJSON(manifestPath), readJSON(specPath)]);
  const helpers = discoverHelpers(makeClient(baseURL, apiKey));
  const plans = [];
  for (const operation of manifest) {
    const definition = operationDefinition(spec, operation);
    const request = await requestFixture(spec, operation, definition);
    const helper = helpers.get(endpointKey(operation.Method, operation.Path));
    plans.push({ operation, definition, request, helper, response: responseFixture(spec, definition, helper) });
  }
  return plans;
}

export async function buildBackendFixtures(settings) {
  const plans = await buildHTTPCasePlans(settings);
  return Promise.all(plans.map(async ({ operation, request, response }) => ({
    operation: operation.OperationID,
    capability: operation.Capability,
    request_media: actualRequestMedia(operation, request.mediaType),
    parameters: Object.fromEntries(Object.entries({ ...request.pathParameters, ...request.query }).map(([key, value]) => [key, String(value)])),
    body_json: actualRequestMedia(operation, request.mediaType).includes('json') ? canonicalJSONBody(operation, request.body) : undefined,
    body_fields: actualRequestMedia(operation, request.mediaType) === 'multipart/form-data' ? Object.keys(request.body ?? {}).sort() : undefined,
    response_status: response.status,
    response_media: response.mediaType,
    response_body: Buffer.from(response.body).toString('base64'),
  })));
}

async function runNegativeCases(client) {
  try {
    await client.embeddings.create({ model: 'text-embedding-test' });
  } catch (error) {
    if (error?.status === 400) return ['missing_required_input'];
    throw new Error(`negative missing-required case returned HTTP ${error?.status ?? 'unknown'}`);
  }
  throw new Error('negative missing-required case was accepted');
}

function makeClient(baseURL, apiKey, defaultHeaders = undefined) {
  return new OpenAI({ apiKey, adminAPIKey: apiKey, baseURL, defaultHeaders, maxRetries: 0, timeout: 10_000 });
}

function discoverHelpers(client) {
  const seen = new Set();
  const helpers = new Map();
  function walk(value, owner, depth) {
    if (!value || typeof value !== 'object' || seen.has(value) || depth > 8) return;
    seen.add(value);
    const prototype = Object.getPrototypeOf(value);
    for (const name of Object.getOwnPropertyNames(prototype ?? {})) {
      if (name === 'constructor' || typeof value[name] !== 'function') continue;
      const source = value[name].toString().replaceAll(/\s+/g, ' ');
      const request = source.match(/this\._client\.(getAPIList|get|post|put|patch|delete)\s*\(/);
      if (!request) continue;
      const literal = source.match(/this\._client\.(?:getAPIList|get|post|put|patch|delete)\s*\(\s*['"]([^'"]+)['"]/)?.[1];
      const template = source.match(/this\._client\.(?:getAPIList|get|post|put|patch|delete)\s*\(\s*path\s*`([^`]+)`/)?.[1];
      const endpoint = (literal ?? template ?? '').replaceAll(/\$\{[^}]+\}/g, '{}');
      const override = source.match(/method:\s*['"](get|post|put|patch|delete)['"]/)?.[1];
      const method = (override ?? (request[1] === 'getAPIList' ? 'get' : request[1])).toUpperCase();
      if (endpoint) helpers.set(`${method} ${endpoint}`, { owner, name, source });
    }
    for (const key of Object.keys(value).sort()) {
      if (key.startsWith('_') || key === 'fetch' || key === 'logger') continue;
      let child;
      try { child = value[key]; } catch { continue; }
      if (child && typeof child === 'object') walk(child, `${owner}.${key}`, depth + 1);
    }
  }
  for (const key of Object.keys(client).sort()) {
    if (key.startsWith('_') || key === 'fetch' || key === 'logger') continue;
    const value = client[key];
    if (value && typeof value === 'object') walk(value, `client.${key}`, 0);
  }
  return helpers;
}

async function invokeHelper(client, helper, request) {
  const resource = helper.owner.slice('client.'.length).split('.').reduce((value, key) => value[key], client);
  const combined = { ...request.query, ...request.body };
  for (const [key, value] of Object.entries(request.pathParameters)) {
    if (helper.source.includes(`\${${key}}`)) combined[key] = value;
  }
  const args = functionParameters(helper.source).map((parameter) => {
    const name = parameter.replace(/\s*=.*$/, '').trim();
    if (name === 'options') return {};
    if (name === 'body') return request.body;
    if (name === 'query') return request.query;
    if (name === 'params') return combined;
    return valueForParameter(name, request.pathParameters);
  });
  return resource[helper.name](...args);
}

async function invokeRaw(client, operation, request) {
  const method = operation.Method.toLowerCase();
  const options = { query: request.query };
  if (!['get', 'delete'].includes(method) && request.body !== undefined) {
    if (request.mediaType === 'multipart/form-data') {
      options.body = toFormData(request.body);
    } else {
      options.body = request.body;
      if (request.mediaType && request.mediaType !== 'application/json') options.headers = { 'Content-Type': request.mediaType };
    }
  }
  return client[method](materializePath(operation.Path, request.pathParameters), options);
}

function functionParameters(source) {
  const start = source.indexOf('(');
  let depth = 0;
  for (let index = start + 1; index < source.length; index += 1) {
    if ('([{'.includes(source[index])) depth += 1;
    if (')]}'.includes(source[index])) {
      if (source[index] === ')' && depth === 0) return splitParameters(source.slice(start + 1, index));
      depth -= 1;
    }
  }
  throw new Error(`cannot parse helper parameters: ${source}`);
}

function splitParameters(value) {
  const result = [];
  let start = 0;
  let depth = 0;
  for (let index = 0; index <= value.length; index += 1) {
    const character = value[index];
    if (character && '([{'.includes(character)) depth += 1;
    if (character && ')]}'.includes(character)) depth -= 1;
    if ((character === ',' && depth === 0) || index === value.length) {
      const parameter = value.slice(start, index).trim();
      if (parameter) result.push(parameter);
      start = index + 1;
    }
  }
  return result;
}

function valueForParameter(name, pathParameters) {
  const normalized = name.replaceAll(/([a-z])([A-Z])/g, '$1_$2').toLowerCase();
  for (const [key, value] of Object.entries(pathParameters)) {
    if (key.toLowerCase() === normalized || key.replaceAll('_', '').toLowerCase() === normalized.replaceAll('_', '')) return value;
  }
  return /version|index|limit|offset/.test(normalized) ? 1 : 'test-id';
}

function operationDefinition(spec, operation) {
  const [path] = operation.Path.split('?');
  const profilePath = spec.paths[operation.Path] ? operation.Path : path;
  const definition = spec.paths[profilePath]?.[operation.Method.toLowerCase()];
  if (!definition || definition.operationId !== operation.OperationID) throw new Error(`${operation.OperationID}: missing from frozen OpenAPI`);
  return { pathItem: spec.paths[profilePath], operation: definition };
}

async function requestFixture(spec, manifest, { pathItem, operation }) {
  const parameters = [...(pathItem.parameters ?? []), ...(operation.parameters ?? [])].map((item) => dereference(spec, item));
  const pathParameters = {};
  const query = {};
  for (const parameter of parameters) {
    const value = schemaFixture(spec, parameter.schema, parameter.name);
    if (parameter.in === 'path') pathParameters[parameter.name] = String(value);
    if (parameter.in === 'query' && parameter.required) query[parameter.name] = value;
  }
  const [, selector] = manifest.Path.split('?');
  if (selector) for (const [key, value] of new URLSearchParams(selector)) query[key] = value;
  const content = operation.requestBody ? dereference(spec, operation.requestBody).content ?? {} : {};
  const mediaType = preferredRequestMedia(Object.keys(content), manifest.OperationID);
  let body;
  if (mediaType) {
    body = schemaFixture(spec, content[mediaType]?.schema, 'body');
    if (!mediaType.includes('json') && mediaType !== 'multipart/form-data') body = new Blob(['fixture'], { type: mediaType });
    if (mediaType === 'multipart/form-data') body = await hydrateFiles(body);
    if (mediaType.includes('json')) body = hydrateBinaryStrings(body);
  }
  return { pathParameters, query, body, mediaType };
}

function responseFixture(spec, { operation }, helper) {
  const statuses = Object.keys(operation.responses ?? {}).filter((status) => /^2\d\d$/.test(status)).sort();
  const status = Number(statuses[0] ?? 200);
  const response = dereference(spec, operation.responses?.[String(status)] ?? {});
  const content = response.content ?? {};
  if (helper?.source.includes('__binaryResponse: true')) {
    return { status, mediaType: 'application/octet-stream', body: Buffer.from('binary-test'), value: undefined };
  }
  const mediaType = preferredResponseMedia(Object.keys(content));
  if (!mediaType) return { status, mediaType: '', body: Buffer.alloc(0), value: undefined };
  if (!mediaType.includes('json')) return { status, mediaType, body: Buffer.from('binary-test'), value: undefined };
  const responseSchema = content[mediaType]?.schema;
  const responseName = responseSchema?.$ref?.split('/').at(-1);
  const value = responseName === 'Response' || responseName === 'BetaResponse'
    ? minimalResponse()
    : responseName === 'ResponseItemList' || responseName === 'BetaResponseItemList'
      ? { object: 'list', data: [], has_more: false, first_id: 'item_first', last_id: 'item_last' }
      : schemaFixture(spec, responseSchema, 'response');
  return { status, mediaType, body: Buffer.from(JSON.stringify(value)), value };
}

export function schemaFixture(spec, input, name = '', depth = 0, references = new Set()) {
  if (!input || depth > 20) return {};
  const schema = input;
  if (schema.$ref) {
    if (references.has(schema.$ref)) return {};
    const next = new Set(references);
    next.add(schema.$ref);
    return schemaFixture(spec, dereference(spec, schema), name, depth + 1, next);
  }
  if (schema.const !== undefined) return schema.const;
  const declaredType = Array.isArray(schema.type) ? schema.type.find((item) => item !== 'null') : schema.type;
  if (schema.example !== undefined && declaredType !== 'object' && declaredType !== 'array' && !schema.properties && !schema.allOf && !schema.oneOf && !schema.anyOf) {
    return schema.example;
  }
  if (schema.default !== undefined) return schema.default;
  if (schema.enum?.length) return schema.enum[0];
  if (schema.allOf?.length) {
    const values = schema.allOf.map((item) => schemaFixture(spec, item, name, depth + 1, references));
    if (values.every((value) => value && typeof value === 'object' && !Array.isArray(value))) return Object.assign({}, ...values);
    return values[0];
  }
  const union = schema.oneOf ?? schema.anyOf;
  const type = declaredType;
  if (type === 'object' || schema.properties || schema.additionalProperties) {
    const result = {};
    const required = new Set(schema.required ?? []);
    const structuralChoice = union?.find((item) => Array.isArray(dereference(spec, item).required));
    for (const key of dereference(spec, structuralChoice).required ?? []) required.add(key);
    const properties = { ...schema.properties, ...dereference(spec, structuralChoice).properties };
    for (const key of required) result[key] = schemaFixture(spec, properties[key] ?? schema.additionalProperties, key, depth + 1, references);
    return result;
  }
  if (union?.length) {
    const choice = schema.discriminator
      ? union.find((item) => dereference(spec, item).type !== 'null') ?? union[0]
      : union.find((item) => dereference(spec, item).type === 'string')
        ?? union.find((item) => dereference(spec, item).type !== 'null')
        ?? union[0];
    const result = schemaFixture(spec, choice, name, depth + 1, references);
    const discriminator = schema.discriminator?.propertyName;
    if (discriminator && result && typeof result === 'object' && !Array.isArray(result) && result[discriminator] === undefined) {
      const selected = dereference(spec, choice);
      const property = dereference(spec, selected.properties?.[discriminator]);
      result[discriminator] = property.const ?? property.enum?.[0] ?? property.example ?? 'test';
    }
    return result;
  }
  if (type === 'array') return Array.from({ length: Math.max(1, schema.minItems ?? 0) }, () => schemaFixture(spec, schema.items ?? {}, name, depth + 1, references));
  if (type === 'integer' || type === 'number') return Math.max(1, schema.minimum ?? 1);
  if (type === 'boolean') return true;
  if (type === 'null') return null;
  if (schema.format === 'binary') {
    if (/image|mask/i.test(name)) return { __fixture_file__: true, name: `${name}.png`, mediaType: 'image/png' };
    if (/video/i.test(name)) return { __fixture_file__: true, name: `${name}.mp4`, mediaType: 'video/mp4' };
    return { __fixture_file__: true, name: `${name || 'fixture'}.wav`, mediaType: 'audio/wav' };
  }
  if (schema.format === 'date') return '2026-01-01';
  if (schema.format === 'date-time') return '2026-01-01T00:00:00Z';
  if (schema.format === 'uri' || schema.format === 'url') return 'https://example.com/test';
  if (schema.format === 'uuid') return '00000000-0000-4000-8000-000000000001';
  if (schema.format === 'base64') return 'dGVzdA==';
  if (/email/i.test(name)) return 'test@example.com';
  if (/model/i.test(name)) return 'gpt-test';
  return 'test';
}

function dereference(spec, value) {
  if (!value?.$ref) return value ?? {};
  return value.$ref.slice(2).split('/').reduce((current, key) => current[key.replaceAll('~1', '/').replaceAll('~0', '~')], spec);
}

async function hydrateFiles(value) {
  if (Array.isArray(value)) return Promise.all(value.map(hydrateFiles));
  if (!value || typeof value !== 'object') return value;
  if (value.__fixture_file__) {
    const contents = value.mediaType === 'audio/wav' ? silenceWAV : Buffer.from('fixture');
    return toFile(contents, value.name, { type: value.mediaType });
  }
  const result = {};
  for (const [key, child] of Object.entries(value)) result[key] = await hydrateFiles(child);
  return result;
}

function hydrateBinaryStrings(value) {
  if (Array.isArray(value)) return value.map(hydrateBinaryStrings);
  if (!value || typeof value !== 'object') return value;
  if (value.__fixture_file__) return 'fixture';
  return Object.fromEntries(Object.entries(value).map(([key, child]) => [key, hydrateBinaryStrings(child)]));
}

function minimalResponse() {
  return {
    id: 'resp_test', object: 'response', created_at: 1, error: null,
    incomplete_details: null, instructions: null, model: 'gpt-test', tools: [],
    output: [], parallel_tool_calls: true, metadata: {}, tool_choice: 'auto',
    temperature: 1, top_p: 1,
  };
}

function toFormData(value) {
  const form = new FormData();
  for (const [key, child] of Object.entries(value ?? {})) appendFormValue(form, key, child);
  return form;
}

function appendFormValue(form, key, value) {
  if (Array.isArray(value)) {
    for (const child of value) appendFormValue(form, `${key}[]`, child);
  } else if (key === 'sdp') {
    form.append(key, new Blob([String(value)], { type: 'application/sdp' }), 'offer.sdp');
  } else if (key === 'session' && value && typeof value === 'object') {
    form.append(key, new Blob([JSON.stringify(value)], { type: 'application/json' }), 'session.json');
  } else if (value instanceof File || value instanceof Blob) {
    form.append(key, value);
  } else if (value && typeof value === 'object') {
    form.append(key, JSON.stringify(value));
  } else {
    form.append(key, String(value));
  }
}

async function assertSDKResult(operation, response, result) {
  if (!response.mediaType) return;
  if (!response.mediaType.includes('json')) {
    const observed = result instanceof Response
      ? Buffer.from(await result.arrayBuffer())
      : Buffer.from(typeof result === 'string' ? result : JSON.stringify(result));
    if (!observed.equals(response.body)) throw new Error(`${operation.OperationID}: SDK non-JSON response mismatch`);
    return;
  }
  if (result === undefined || result === null || typeof result !== 'object') throw new Error(`${operation.OperationID}: SDK did not parse a JSON object`);
  if (Array.isArray(response.value?.data) && !Array.isArray(result.data)) throw new Error(`${operation.OperationID}: SDK result is missing list data`);
  if (response.value?.id !== undefined && result.id !== response.value.id) throw new Error(`${operation.OperationID}: SDK result id mismatch`);
  if (response.value?.deleted !== undefined && result.deleted !== response.value.deleted) throw new Error(`${operation.OperationID}: SDK result deletion state mismatch`);
}

function materializePath(path, parameters) {
  return path.split('?')[0].replaceAll(/\{([^}]+)\}/g, (_, name) => encodeURIComponent(parameters[name] ?? 'test-id'));
}

function endpointKey(method, path) { return `${method} ${path.replaceAll(/\{[^}]+\}/g, '{}')}`; }
function actualRequestMedia(operation, generatedMedia) {
  if (operation.OperationID === 'CreateContainerFile' || operation.OperationID === 'CreateVideoRemix') return 'application/json';
  return generatedMedia;
}
function canonicalJSONBody(operation, body) {
  let result = body;
  if (operation.OperationID === 'createEmbedding') result = { ...result, encoding_format: result.encoding_format ?? 'base64' };
  const [, selector] = operation.Path.split('?');
  if (selector && result && typeof result === 'object') result = { ...Object.fromEntries(new URLSearchParams(selector)), ...result };
  return result;
}
function preferredRequestMedia(mediaTypes, operation) {
  if (operation === 'create-realtime-call' && mediaTypes.includes('application/sdp')) return 'application/sdp';
  const preferJSON = operation === 'CreateSkill' || operation === 'CreateSkillVersion';
  const order = preferJSON
    ? ['application/json', 'multipart/form-data', 'application/octet-stream']
    : ['multipart/form-data', 'application/json', 'application/octet-stream'];
  return order.find((item) => mediaTypes.includes(item)) ?? mediaTypes[0] ?? '';
}
function preferredResponseMedia(mediaTypes) { return mediaTypes.find((item) => item === 'application/json') ?? mediaTypes.find((item) => item.includes('json')) ?? mediaTypes.find((item) => item !== 'text/event-stream') ?? mediaTypes[0] ?? ''; }
async function readJSON(path) { return JSON.parse(await fs.readFile(path, 'utf8')); }
