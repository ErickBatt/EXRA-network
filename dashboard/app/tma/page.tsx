// TMA route — delegates to TMAApp component which uses the secure server-side proxy.
// NODE_SECRET never reaches the browser; all /api/tma/* calls go through /next-tma/* proxy.
import TMAApp from './TMAApp'
export default TMAApp
