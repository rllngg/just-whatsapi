const express = require('express');
const bodyParser = require('body-parser');
const app = express();
const port = 3000;

app.use(bodyParser.json({ limit: '50mb' }));
app.use(bodyParser.urlencoded({ extended: true, limit: '50mb' }));

// Store received webhooks in memory
const webhooks = [];

// Webhook receiver endpoint
app.post('/webhook', (req, res) => {
  const timestamp = new Date().toISOString();
  const webhook = {
    timestamp,
    headers: req.headers,
    body: req.body
  };
  
  webhooks.push(webhook);
  
  console.log(`[${timestamp}] Webhook received:`);
  console.log(JSON.stringify(webhook, null, 2));
  
  res.status(200).json({ 
    success: true, 
    message: 'Webhook received',
    timestamp 
  });
});

// Get all received webhooks
app.get('/webhooks', (req, res) => {
  res.json({
    count: webhooks.length,
    webhooks
  });
});

// Clear all webhooks
app.delete('/webhooks', (req, res) => {
  const count = webhooks.length;
  webhooks.length = 0;
  res.json({
    success: true,
    message: `Cleared ${count} webhooks`
  });
});

// Health check
app.get('/health', (req, res) => {
  res.json({ status: 'ok' });
});

app.listen(port, '0.0.0.0', () => {
  console.log(`Webhook receiver listening on port ${port}`);
});
