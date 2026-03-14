// This file abstracts all API communication.
// Components should import from here, not use fetch() directly.

import { writable } from 'svelte/store';

// A private, non-exported store to hold the token in memory.
const accessToken = writable<string | null>(null);

/**
 * A reactive store that holds the current authentication state.
 * Components can subscribe to this to react to login/logout events.
 */
export const isAuthenticated = writable<boolean>(false);

const apiFetch = async (url: string, options: RequestInit = {}) => {
	let token: string | null = null;
	accessToken.subscribe(value => token = value)();

	const headers = new Headers(options.headers);
	if (token) {
		headers.set('Authorization', `Bearer ${token}`);
	}
	headers.set('Content-Type', 'application/json');

	const fullUrl = import.meta.env.DEV ? url : `/api${url.substring(4)}`;

	const response = await fetch(fullUrl, { ...options, headers });

	if (response.status === 401) {
		const refreshed = await refreshToken();
		if (refreshed) {
			return apiFetch(url, options);
		} else {
			auth.logout();
		}
	}

	return response;
};

const refreshToken = async (): Promise<boolean> => {
	const response = await fetch('/api/auth/refresh', { method: 'POST' });
	if (response.ok) {
		const data = await response.json();
		accessToken.set(data.access_token);
		return true;
	}
	return false;
};

export const auth = {
	register: async (username: string, password: string): Promise<Response> => {
		return apiFetch('/api/auth/register', {
			method: 'POST',
			body: JSON.stringify({ username, password }),
		});
	},

	login: async (username: string, password: string): Promise<boolean> => {
		const response = await apiFetch('/api/auth/login', {
			method: 'POST',
			body: JSON.stringify({ username, password }),
		});

		if (response.ok) {
			const data = await response.json();
			accessToken.set(data.access_token);
			isAuthenticated.set(true);
			return true;
		}
		return false;
	},

	logout: async (): Promise<void> => {
		await apiFetch('/api/auth/logout', { method: 'POST' });
		accessToken.set(null);
		isAuthenticated.set(false);
	},
};
