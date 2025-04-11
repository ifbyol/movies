/**
 * Custom fetch function that propagates baggage headers from incoming requests to outgoing requests
 * @param {string} url - The URL to fetch
 * @param {Object} options - Fetch options
 * @returns {Promise<Response>} - The fetch response
 */
export const fetchWithBaggage = (url, options = {}) => {
  // Get baggage header from document cookies if available
  const cookies = document.cookie.split(';');
  let baggageHeader = '';
  
  for (const cookie of cookies) {
    const [name, value] = cookie.trim().split('=');
    if (name === 'baggage') {
      baggageHeader = decodeURIComponent(value);
      break;
    }
  }

  // Prepare headers
  const headers = options.headers || {};
  
  // Add baggage header if it exists
  if (baggageHeader) {
    headers.baggage = baggageHeader;
  }

  // Return fetch with updated headers
  return fetch(url, {
    ...options,
    headers
  });
};