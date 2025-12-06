/**
 * nettune-mcp - MCP stdio wrapper for nettune
 *
 * This wrapper downloads and manages the nettune binary, then spawns
 * the client in MCP stdio mode, transparently passing through stdin/stdout.
 */

import { parseArgs, buildClientArgs, validateOptions } from './cli.js';
import { detectPlatform } from './platform.js';
import { BinaryManager, createDefaultConfig } from './binary-manager.js';
import { spawnNettuneClient } from './spawner.js';

/**
 * Log a message to stderr to avoid polluting MCP stdout
 */
function log(message: string): void {
  console.error(`[nettune-mcp] ${message}`);
}

/**
 * Log an error message to stderr
 */
function logError(message: string): void {
  console.error(`[nettune-mcp] Error: ${message}`);
}

/**
 * Main entry point
 */
async function main(): Promise<void> {
  try {
    // Parse command line arguments
    const options = parseArgs();

    // Validate options
    validateOptions(options);

    if (options.verbose) {
      log(`Server: ${options.server}`);
      log(`Version: ${options.version}`);
    }

    // Detect platform
    const platform = detectPlatform();
    if (options.verbose) {
      log(`Platform: ${platform.os}-${platform.arch}`);
    }

    // Initialize binary manager
    const binaryConfig = createDefaultConfig({
      version: options.version,
    });
    const binaryManager = new BinaryManager(binaryConfig);

    // Ensure binary is available
    const binaryPath = await binaryManager.ensureBinary(platform);
    if (options.verbose) {
      log(`Binary path: ${binaryPath}`);
    }

    // Build client arguments
    const clientArgs = buildClientArgs(options);

    // Spawn the nettune client
    const exitCode = await spawnNettuneClient({
      binaryPath,
      args: clientArgs,
    });

    // Exit with the same code as the child process
    process.exit(exitCode);
  } catch (error) {
    logError(error instanceof Error ? error.message : String(error));
    process.exit(1);
  }
}

// Run main function
main();
