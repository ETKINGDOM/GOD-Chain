// Transport availability is separate from malformed data, denied requests and
// a different network. Only explicitly transient read failures may be retried.
export class QueryUnavailableError extends Error {
  constructor(){super('Testnet read temporarily unavailable.');}
}
export const transientHTTP=status=>[408,502,503,504].includes(status);
export const isQueryUnavailable=error=>error instanceof QueryUnavailableError;
