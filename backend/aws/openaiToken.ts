import { APIGatewayEvent, APIGatewayProxyResult } from "aws-lambda";

export async function handler(
  event: APIGatewayEvent
): Promise<APIGatewayProxyResult> {
  const openaiApiKey = process.env.OPENAI_API_KEY;

  if (!openaiApiKey) {
    return {
      statusCode: 500,
      body: "OpenAI API key not found",
    };
  }

  return {
    statusCode: 200,
    body: JSON.stringify({ apiKey: openaiApiKey }),
  };
}
