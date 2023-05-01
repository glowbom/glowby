import https from 'https';
import http from 'http';

export async function handler(event) {
  const imageUrl = event.queryStringParameters?.imageUrl;

  const headers = {
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Methods': 'POST',
    'Access-Control-Allow-Headers': 'Content-Type'
  };

  if (!imageUrl) {
    return {
      statusCode: 400,
      headers,
      body: JSON.stringify({
        success: false,
        message: 'Image URL parameter is required',
      }),
    };
  }

  const httpModule = imageUrl.startsWith('https') ? https : http;

  try {
    const base64Image = await new Promise((resolve, reject) => {
      httpModule.get(imageUrl, (res) => {
        if (res.statusCode !== 200) {
          reject(new Error(`Request failed with status code ${res.statusCode}`));
          return;
        }

        const chunks = [];
        res.on('data', (chunk) => chunks.push(chunk));
        res.on('end', () => {
          const buffer = Buffer.concat(chunks);
          const base64 = buffer.toString('base64');
          resolve(base64);
        });
      }).on('error', (err) => {
        reject(err);
      });
    });

    return {
      statusCode: 200,
      headers,
      body: JSON.stringify({ success: true, base64Image }),
    };
  } catch (error) {
    console.error('Error fetching image:', error);
    return {
      statusCode: 500,
      headers,
      body: JSON.stringify({
        success: false,
        message: `Failed to fetch image: ${error.message}`,
      }),
    };
  }
}
