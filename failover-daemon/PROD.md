# running in production

hasn't been tested, need following changes

- hash token, compare hashed
- check auth before returning 404
- set state.targetServer AFTER route change command succeeded
- maybe not store state in memory but fetch on-demand
- different providers (switch routing APIs) and config for each provider
- needs webinterface and audit log
