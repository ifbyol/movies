/**
 * Middleware to capture baggage header from incoming requests and store it in a cookie
 * This should be included in the index.jsx file
 */
export const setupBaggageCapture = () => {
  // Function to extract headers from the current request
  const captureHeaders = () => {
    // Try to get the baggage header from the server-side rendered page
    const headerElements = document.getElementsByTagName('meta');
    for (let i = 0; i < headerElements.length; i++) {
      const element = headerElements[i];
      if (element.getAttribute('name') === 'baggage') {
        const baggageValue = element.getAttribute('content');
        if (baggageValue) {
          // Store the baggage header in a cookie
          document.cookie = `baggage=${encodeURIComponent(baggageValue)}; path=/`;
          return;
        }
      }
    }
  };

  // Capture headers when the page loads
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', captureHeaders);
  } else {
    captureHeaders();
  }

  // Override the fetch function to propagate baggage headers
  const originalFetch = window.fetch;
  window.fetch = function(url, options = {}) {
    // Get baggage header from cookie
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
    return originalFetch(url, {
      ...options,
      headers
    });
  };
};