export function searchServiceConfig(configuration) {
  if (!configuration.ports.kafka && !configuration.ports.es) return undefined;
  if (
    !configuration.ports.kafka ||
    !configuration.ports.es ||
    !configuration.passwords.searchCursorKey
  )
    throw Error('SEARCH_ENVIRONMENT_STATE_INVALID');
  return {
    owner: configuration.owner,
    brokers: [`127.0.0.1:${configuration.ports.kafka}`],
    elasticEndpoint: `http://127.0.0.1:${configuration.ports.es}`,
    topic: 'collab-publications-v1',
    group: configuration.project + '-search',
    alias: 'collab-publications',
    initialIndex: 'collab-publications-g1',
    cursorKey: configuration.passwords.searchCursorKey,
    cursorTTLSeconds: 30,
  };
}
