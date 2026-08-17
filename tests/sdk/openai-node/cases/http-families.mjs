import { administrationCapabilities } from './administration.mjs';
import { agentCapabilities } from './agents.mjs';
import { chatkitCapabilities } from './chatkit.mjs';
import { dataCapabilities } from './data.mjs';
import { generationCapabilities } from './generation.mjs';
import { mediaCapabilities } from './media.mjs';

const families = [
  ['generation', generationCapabilities], ['media', mediaCapabilities], ['data', dataCapabilities],
  ['agents', agentCapabilities], ['administration', administrationCapabilities], ['chatkit', chatkitCapabilities],
];

export function caseFamilyFor(capability) {
  const matches = families.filter(([, capabilities]) => capabilities.has(capability));
  if (matches.length !== 1) throw new Error(`capability ${capability} belongs to ${matches.length} case families`);
  return matches[0][0];
}
