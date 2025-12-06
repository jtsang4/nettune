import { describe, expect, test } from 'bun:test';
import { buildClientArgs, validateOptions } from '../src/cli';
import type { CLIOptions } from '../src/types';

describe('buildClientArgs', () => {
  test('builds correct args with all options', () => {
    const options: CLIOptions = {
      apiKey: 'test-key',
      server: 'http://localhost:9876',
      version: 'latest',
      verbose: false,
    };

    const args = buildClientArgs(options);

    expect(args).toContain('--api-key');
    expect(args).toContain('test-key');
    expect(args).toContain('--server');
    expect(args).toContain('http://localhost:9876');
  });

  test('includes all required arguments in correct order', () => {
    const options: CLIOptions = {
      apiKey: 'my-api-key',
      server: 'http://example.com:9876',
      version: 'v1.0.0',
      verbose: true,
    };

    const args = buildClientArgs(options);

    // Check that api-key comes before its value
    const apiKeyIndex = args.indexOf('--api-key');
    expect(apiKeyIndex).toBeGreaterThanOrEqual(0);
    expect(args[apiKeyIndex + 1]).toBe('my-api-key');

    // Check that server comes before its value
    const serverIndex = args.indexOf('--server');
    expect(serverIndex).toBeGreaterThanOrEqual(0);
    expect(args[serverIndex + 1]).toBe('http://example.com:9876');
  });
});

describe('validateOptions', () => {
  test('accepts valid options', () => {
    const options: CLIOptions = {
      apiKey: 'test-key',
      server: 'http://localhost:9876',
      version: 'latest',
      verbose: false,
    };

    expect(() => validateOptions(options)).not.toThrow();
  });

  test('throws on empty API key', () => {
    const options: CLIOptions = {
      apiKey: '',
      server: 'http://localhost:9876',
      version: 'latest',
      verbose: false,
    };

    expect(() => validateOptions(options)).toThrow('API key is required');
  });

  test('throws on invalid server URL', () => {
    const options: CLIOptions = {
      apiKey: 'test-key',
      server: 'not-a-valid-url',
      version: 'latest',
      verbose: false,
    };

    expect(() => validateOptions(options)).toThrow('Invalid server URL');
  });
});
