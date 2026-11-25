import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { validateMigration } from './migration-checksum.mjs';
import { validateIndexManifest } from './migration-indexes.mjs';
import { validateTableManifest, validateCheckManifest } from './migration-tables.mjs';

export function readMigrationChain(folder) {
  const manifests = [
    'schema-manifest.json',
    ...readdirSync(folder)
      .filter((name) => /^\d{3}-manifest\.json$/.test(name))
      .sort(),
  ];
  const chain = [];
  for (const [index, file] of manifests.entries()) {
    const version = index + 1;
    const manifest = JSON.parse(readFileSync(resolve(folder, file)));
    const prefix = String(version).padStart(3, '0');
    if (
      manifest.version !== version ||
      (version > 1 &&
        (file !== prefix + '-manifest.json' ||
          manifest.parentSHA256 !== chain[index - 1].manifest.sha256))
    )
      throw Error('MIGRATION_VERSION_MISMATCH');
    const bytes = validateMigration(readFileSync(resolve(folder, prefix + '.up.sql')), manifest);
    if (version > 1) {
      if ([manifest.indexes, manifest.tables, manifest.checks].filter(Boolean).length !== 1)
        throw Error('MIGRATION_MANIFEST_INVALID');
      if (manifest.indexes) validateIndexManifest(manifest);
      else if (manifest.checks) validateCheckManifest(manifest);
      else validateTableManifest(manifest);
    }
    chain.push({ manifest, bytes });
  }
  return chain;
}
