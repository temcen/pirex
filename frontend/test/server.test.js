const request = require('supertest');
const { app } = require('../server');

describe('Frontend Server', () => {
  test('Health endpoint should return 200', async () => {
    const response = await request(app)
      .get('/health')
      .expect(200);
    
    expect(response.body.status).toBe('healthy');
    expect(response.body.service).toBe('recommendation-frontend');
  });
  
  test('Root endpoint should serve HTML', async () => {
    const response = await request(app)
      .get('/')
      .expect(200);
    
    expect(response.headers['content-type']).toMatch(/text\/html/);
  });
  
  test('Admin endpoint should serve HTML', async () => {
    const response = await request(app)
      .get('/admin')
      .expect(200);
    
    expect(response.headers['content-type']).toMatch(/text\/html/);
  });
  
  test('API proxy should handle requests', async () => {
    // This test would need the backend running
    // For now, just test that the route exists
    const response = await request(app)
      .get('/api/v1/health')
      .expect(500); // Expected since backend isn't running
    
    expect(response.body.error).toBe('Backend service unavailable');
  });
});