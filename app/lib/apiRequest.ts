export type FetchLike = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

async function readErrorMessage(response: Response): Promise<string> {
  const contentType = response.headers.get('content-type') ?? '';

  if (contentType.includes('application/json')) {
    const payload = await response
      .json()
      .catch(() => null) as
      | {error?: unknown; message?: unknown; code?: unknown}
      | null;
    const apiMessage =
      typeof payload?.error === 'string'
        ? payload.error
        : typeof payload?.message === 'string'
          ? payload.message
          : null;
    if (apiMessage && apiMessage.trim()) {
      return apiMessage;
    }
  }

  const rawText = (await response.text().catch(() => '')).trim();
  if (rawText) {
    return rawText;
  }

  return `Request failed (${response.status})`;
}

export async function requestFromCandidates<T>(
  baseUrls: readonly string[],
  path: string,
  init: RequestInit = {},
  token?: string,
  fetchImpl: FetchLike = fetch,
): Promise<T> {
  const requestUrls = baseUrls.map((baseUrl) => `${baseUrl}${path}`);
  let response: Response | null = null;
  let lastNetworkError: string | null = null;

  for (const requestUrl of requestUrls) {
    try {
      response = await fetchImpl(requestUrl, {
        ...init,
        headers: {
          'Content-Type': 'application/json',
          ...(token ? {Authorization: `Bearer ${token}`} : {}),
          ...(init.headers ?? {}),
        },
      });
      break;
    } catch (error) {
      lastNetworkError = error instanceof Error ? error.message : 'Unknown network error';
    }
  }

  if (!response) {
    throw new Error(
      `Network error: ${lastNetworkError ?? 'Request failed'} (${requestUrls.join(' | ')})`,
    );
  }

  if (!response.ok) {
    throw new Error(await readErrorMessage(response));
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return response.json() as Promise<T>;
}

export function requestFromSingleBase<T>(
  baseUrl: string,
  path: string,
  init: RequestInit = {},
  token?: string,
  fetchImpl: FetchLike = fetch,
): Promise<T> {
  return requestFromCandidates<T>([baseUrl], path, init, token, fetchImpl);
}
