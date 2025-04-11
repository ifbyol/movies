import { hot } from 'react-hot-loader/root';
import React from 'react';
import { render } from 'react-dom';

import App from './App';
import './index.css';
import { setupBaggageCapture } from './utils/baggageMiddleware';

// Setup baggage header capture and propagation
setupBaggageCapture();

if (module.hot) {
  module.hot.accept();
}

const Root = hot(App);
render(<Root />, document.getElementById('root'));
