// This file abstracts all API communication.
// Components should import from here, not use fetch() directly.

import { writable } from 'svelte/store';

/**
 * A reactive store that holds the current authentication state.
 * Components can subscribe to this to react to login/logout events.
 */
export const isAuthenticated = writable<boolean>(false);

let csrfToken: string | null = null;

export type Node = {
	id: string;
	name?: string;
	last_seen: number;
	connected: boolean;
};

export type Command = {
	id: string;
	node_id: string;
	action: string;
	target_id?: string;
	status: string;
	success?: boolean;
	message?: string;
	created_at: number;
	completed_at?: number;
};

export type CommandResponse = {
	command_id: string;
};

export const apiFetch = async (url: string, options: RequestInit = {}) => {
	const headers = new Headers(options.headers);
	headers.set('Content-Type', 'application/json');

	const method = (options.method ?? 'GET').toUpperCase();
	const needsCSRF =
		['POST', 'PUT', 'PATCH', 'DELETE'].includes(method) &&
		!url.startsWith('/api/auth/login') &&
		!url.startsWith('/api/auth/register');

	if (needsCSRF) {
		if (!csrfToken) {
			await loadCSRFToken();
		}
		if (csrfToken) {
			headers.set('X-CSRF-Token', csrfToken);
		}
	}

	const fullUrl = import.meta.env.DEV ? url : `/api${url.substring(4)}`;

	const response = await fetch(fullUrl, { ...options, headers, credentials: 'same-origin' });

	if (response.status === 401) {
		isAuthenticated.set(false);
	}

	return response;
};

const loadCSRFToken = async (): Promise<void> => {
	const response = await fetch('/api/auth/csrf', { credentials: 'same-origin' });
	if (response.ok) {
		const data = await response.json();
		csrfToken = data.csrf_token;
	}
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
			csrfToken = data.csrf_token;
			isAuthenticated.set(true);
			return true;
		}
		return false;
	},

	logout: async (): Promise<void> => {
		await apiFetch('/api/auth/logout', { method: 'POST' });
		csrfToken = null;
		isAuthenticated.set(false);
	},
};

export const panel = {
	nodes: async (): Promise<Node[]> => {
		const response = await apiFetch('/api/nodes');
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},

	createContainer: async (nodeID: string, name: string, image: string): Promise<CommandResponse> => {
		const response = await apiFetch(`/api/nodes/${nodeID}/containers`, {
			method: 'POST',
			body: JSON.stringify({ name, image }),
		});
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},

	containerAction: async (
		nodeID: string,
		containerID: string,
		action: 'start' | 'stop' | 'remove',
	): Promise<CommandResponse> => {
		const response = await apiFetch(`/api/nodes/${nodeID}/containers/${containerID}/${action}`, {
			method: 'POST',
		});
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},

	command: async (commandID: string): Promise<Command> => {
		const response = await apiFetch(`/api/commands/${commandID}`);
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},
};
