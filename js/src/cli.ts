import { Command } from 'commander';
import pkg from '../package.json' assert { type: 'json' };
import type { CLIOptions } from './types.js';

/**
 * Package version - will be replaced during build
 */
const VERSION = pkg.version as string;

/**
 * Parse command line arguments and return CLI options
 */
export function parseArgs(): CLIOptions {
  const program = new Command();

  program
    .name('nettune-mcp')
    .description('MCP stdio wrapper for nettune TCP network optimization tool')
    .version(VERSION)
    .requiredOption(
      '-k, --api-key <key>',
      'API key for server authentication (required)'
    )
    .option(
      '-s, --server <url>',
      'Server URL',
      'http://127.0.0.1:9876'
    )
    .option(
      '--version-tag <version>',
      'Specific nettune version to use',
      'latest'
    )
    .option(
      '-v, --verbose',
      'Enable verbose logging',
      false
    )
    .parse(process.argv);

  const opts = program.opts();

  return {
    apiKey: opts.apiKey as string,
    server: opts.server as string,
    version: opts.versionTag as string,
    verbose: opts.verbose as boolean,
  };
}

/**
 * Build arguments array for the nettune client command
 */
export function buildClientArgs(options: CLIOptions): string[] {
  const args: string[] = [];

  // Required: API key
  args.push('--api-key', options.apiKey);

  // Server URL
  args.push('--server', options.server);

  return args;
}

/**
 * Validate CLI options
 * @throws Error if validation fails
 */
export function validateOptions(options: CLIOptions): void {
  // API key is required and enforced by commander
  if (!options.apiKey || options.apiKey.trim() === '') {
    throw new Error('API key is required (--api-key)');
  }

  // Validate server URL format
  try {
    new URL(options.server);
  } catch {
    throw new Error(`Invalid server URL: ${options.server}`);
  }
}
