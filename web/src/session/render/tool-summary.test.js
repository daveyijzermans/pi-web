import { describe, expect, it } from 'vitest';
import { resolveToolResult, resolveToolStatus } from './tool-summary.js';

function makeModel(results = []) {
  const entries = [];
  for (const r of results) {
    entries.push({
      id: r.id,
      type: 'message',
      message: {
        role: 'toolResult',
        toolCallId: r.callId,
        isError: r.isError ?? false,
        content: [],
      },
    });
  }
  return { entries };
}

describe('resolveToolResult', () => {
  it('returns the message object when result exists', () => {
    const model = makeModel([{ id: 'r1', callId: 'tc1' }]);
    const result = resolveToolResult(model, 'tc1');
    expect(result).toBeTruthy();
    expect(result.role).toBe('toolResult');
    expect(result.toolCallId).toBe('tc1');
  });

  it('returns null when no result found', () => {
    const model = makeModel([]);
    expect(resolveToolResult(model, 'tc1')).toBeNull();
  });

  it('returns null for null model', () => {
    expect(resolveToolResult(null, 'tc1')).toBeNull();
  });

  it('preserves details and isError on the message', () => {
    const entries = [
      {
        id: 'r1',
        type: 'message',
        message: {
          role: 'toolResult',
          toolCallId: 'tc1',
          isError: true,
          content: [],
          details: { diff: '+added\n-removed' },
        },
      },
    ];
    const result = resolveToolResult({ entries }, 'tc1');
    expect(result.isError).toBe(true);
    expect(result.details.diff).toBe('+added\n-removed');
  });
});

describe('resolveToolResult with toolResultMap', () => {
  it('prefers the map lookup over scanning entries', () => {
    const message = { role: 'toolResult', toolCallId: 'tc1', isError: false, content: [] };
    const model = { entries: [], toolResultMap: new Map([['tc1', message]]) };
    expect(resolveToolResult(model, 'tc1')).toBe(message);
  });

  it('returns null from the map path when the call id is absent', () => {
    const model = { entries: [], toolResultMap: new Map() };
    expect(resolveToolResult(model, 'tc-missing')).toBeNull();
  });
});

describe('resolveToolStatus', () => {
  it('returns success when result exists without error', () => {
    const model = makeModel([{ id: 'r1', callId: 'tc1' }]);
    expect(resolveToolStatus(model, 'tc1')).toBe('success');
  });

  it('returns error when result has isError', () => {
    const model = makeModel([{ id: 'r1', callId: 'tc1', isError: true }]);
    expect(resolveToolStatus(model, 'tc1')).toBe('error');
  });

  it('returns pending when no result found', () => {
    const model = makeModel([]);
    expect(resolveToolStatus(model, 'tc1')).toBe('pending');
  });

  it('returns pending for null model', () => {
    expect(resolveToolStatus(null, 'tc1')).toBe('pending');
  });
});
