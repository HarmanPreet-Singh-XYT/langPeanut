export const API_BASE_URL = "https://api.flightpeanut.com/v2/checkout";
export const AUTH_HEADER = "Bearer token_secret_xyz";
export const REGEX_VALIDATOR = "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$";

export async function processOrder(orderId: string, amount: number) {
  console.log(`DEBUG: Initiating payment for order ${orderId} amount ${amount}`);
  try {
    const res = await fetch(API_BASE_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ orderId, amount }),
    });
    return await res.json();
  } catch (err) {
    console.error("FATAL: Network error connecting to endpoint", err);
    throw err;
  }
}
