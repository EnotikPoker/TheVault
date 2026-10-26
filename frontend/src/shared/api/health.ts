export async function checkHealth(signal?: AbortSignal): Promise<void> {
  const timeout = AbortSignal.timeout(5000);
  const response = await fetch('/api/health', {
    signal: signal ? AbortSignal.any([signal, timeout]) : timeout,
  });
  if (!response.ok) throw new Error(`Health check failed: ${response.status}`);

  const body: unknown = await response.json();
  if (typeof body !== 'object' || body === null || !('status' in body) || body.status !== 'ok') {
    throw new Error('Unexpected health response');
  }
}
