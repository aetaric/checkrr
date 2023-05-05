/**
 * @param {string} url
 * @param {RequestInit} options
 */
const jsonFetch = async (url, options) => {
  const response = await fetch(url, options);
  if (!response.ok) {
    throw new Error(
      `HTTP error! status: ${response.status}, ${await response.text()}`
    );
  }
  const json = await response.json();

  return json;
};

const get = (url, options) => jsonFetch(url, { ...options, method: 'GET' });

const post = (url, body, options) =>
  jsonFetch(url, { ...options, body: JSON.stringify(body), method: 'POST' });

export default {
  get,
  post,
};
