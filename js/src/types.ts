/**
 * Supported operating systems
 */
export type OS = 'darwin' | 'linux' | 'win32';

/**
 * Supported CPU architectures
 */
export type Arch = 'x64' | 'arm64';

/**
 * Platform information for binary selection
 */
export interface PlatformInfo {
  os: OS;
  arch: Arch;
}

/**
 * Configuration for the binary manager
 */
export interface BinaryManagerConfig {
  /** Directory to cache downloaded binaries, defaults to ~/.cache/nettune */
  cacheDir: string;
  /** GitHub repository in format "owner/repo" */
  githubRepo: string;
  /** Version to download, defaults to "latest" */
  version: string;
}

/**
 * GitHub release asset information
 */
export interface ReleaseAsset {
  name: string;
  browser_download_url: string;
  size: number;
}

/**
 * GitHub release information
 */
export interface GithubRelease {
  tag_name: string;
  assets: ReleaseAsset[];
}

/**
 * Options for spawning the nettune client process
 */
export interface SpawnOptions {
  /** Path to the nettune binary */
  binaryPath: string;
  /** Arguments to pass to the client command */
  args: string[];
  /** Additional environment variables */
  env?: Record<string, string>;
}

/**
 * CLI options parsed from command line arguments
 */
export interface CLIOptions {
  /** API key for server authentication (required) */
  apiKey: string;
  /** Server URL, defaults to http://127.0.0.1:9876 */
  server: string;
  /**
   * MCP server name identifier (deprecated, no longer used by the Go client)
   * Kept optional for backwards compatibility with older callers.
   */
  mcpName?: string;
  /** Specific version to use */
  version: string;
  /** Enable verbose/debug logging */
  verbose: boolean;
}

/**
 * Checksum information for verifying binary integrity
 */
export interface ChecksumInfo {
  /** SHA256 hash of the binary */
  sha256: string;
  /** Name of the binary file */
  filename: string;
}
