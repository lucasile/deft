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
	created_at: number;
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
	server_id?: string;
};

export type Container = {
	id: string;
	node_id: string;
	name?: string;
	image?: string;
	status?: string;
};

export type Server = {
	id: string;
	name: string;
	node_id: string;
	container_id?: string;
	image: string;
	status: string;
	desired_config_json: string;
	created_at: number;
	updated_at: number;
};

export type ContainerPortMapping = {
	host_port: number;
	container_port: number;
	protocol: 'tcp' | 'udp';
};

export type ContainerEnvVar = {
	key: string;
	value: string;
};

export type ContainerVolumeMount = {
	host_path: string;
	container_path: string;
	read_only?: boolean;
};

export type CreateContainerConfig = {
	ports?: ContainerPortMapping[];
	env?: ContainerEnvVar[];
	volumes?: ContainerVolumeMount[];
	restart_policy?: string;
};

export type RecipeInput = {
	key: string;
	label: string;
	type: 'string' | 'number' | 'select';
	default?: string | number | boolean;
	required: boolean;
	editable_by: 'user' | 'admin';
	min?: number;
	max?: number;
	options?: string[];
};

export type Recipe = {
	id: string;
	name: string;
	description: string;
	version: string;
	source: string;
	enabled: boolean;
	inputs: RecipeInput[];
};

export type PanelEventName = 'nodes.changed' | 'command.updated' | 'containers.changed';

export type PanelEventPayload = {
	node_id?: string;
	command_id?: string;
	container_id?: string;
};

export type LogChunkPayload = {
	node_id: string;
	stream_id: string;
	container_id: string;
	data?: string;
	eof?: boolean;
	error?: string;
};

export type LogStreamResponse = {
	stream_id: string;
};

export type JoinRequestReview = {
	id: string;
	node_name?: string;
	verification_code: string;
	expires_at: number;
	status: string;
};

export type JoinApprovalResult = {
	node_id: string;
	panel_addr: string;
};

export type JoinToken = {
	token: string;
	expires_at: number;
};

export type JoinTokenInfo = {
	id: string;
	node_name?: string;
	created_at: number;
	expires_at: number;
	used_at?: number;
	revoked_at?: number;
	used_by_node_id?: string;
	status: string;
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

	removeNode: async (nodeID: string): Promise<void> => {
		const response = await apiFetch(`/api/nodes/${nodeID}`, {
			method: 'DELETE',
		});
		if (!response.ok) {
			throw new Error(await response.text());
		}
	},

	stopNode: async (nodeID: string): Promise<CommandResponse> => {
		const response = await apiFetch(`/api/nodes/${nodeID}/stop`, {
			method: 'POST',
		});
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},

	createContainer: async (nodeID: string, name: string, image: string, config: CreateContainerConfig = {}): Promise<CommandResponse> => {
		const response = await apiFetch(`/api/nodes/${nodeID}/containers`, {
			method: 'POST',
			body: JSON.stringify({ name, image, ...config }),
		});
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},

	recipes: async (): Promise<Recipe[]> => {
		const response = await apiFetch('/api/recipes');
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},

	createServerFromRecipe: async (nodeID: string, recipeID: string, recipeValues: Record<string, string | number | boolean>): Promise<CommandResponse> => {
		const response = await apiFetch(`/api/nodes/${nodeID}/containers`, {
			method: 'POST',
			body: JSON.stringify({ recipe_id: recipeID, recipe_values: recipeValues }),
		});
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},

	containers: async (nodeID: string): Promise<Container[]> => {
		const response = await apiFetch(`/api/nodes/${nodeID}/containers`);
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

	containerLogs: async (nodeID: string, containerID: string): Promise<CommandResponse> => {
		const response = await apiFetch(`/api/nodes/${nodeID}/containers/${containerID}/logs`, {
			method: 'POST',
		});
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},

	createContainerLogStream: async (nodeID: string, containerID: string): Promise<LogStreamResponse> => {
		const response = await apiFetch(`/api/nodes/${nodeID}/containers/${containerID}/logs/stream`, {
			method: 'POST',
		});
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},

	containerLogStream: (nodeID: string, containerID: string, streamID: string): EventSource => {
		return new EventSource(
			`/api/nodes/${nodeID}/containers/${containerID}/logs/stream?stream_id=${encodeURIComponent(streamID)}`,
			{ withCredentials: true },
		);
	},

	command: async (commandID: string): Promise<Command> => {
		const response = await apiFetch(`/api/commands/${commandID}`);
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},

	commands: async (limit = 100): Promise<Command[]> => {
		const response = await apiFetch(`/api/commands?limit=${limit}`);
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},

	servers: async (): Promise<Server[]> => {
		const response = await apiFetch('/api/servers');
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},

	server: async (serverID: string): Promise<Server> => {
		const response = await apiFetch(`/api/servers/${serverID}`);
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},

	serverAction: async (serverID: string, action: 'start' | 'stop' | 'restart' | 'remove'): Promise<CommandResponse> => {
		const response = await apiFetch(`/api/servers/${serverID}/${action}`, {
			method: 'POST',
		});
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},

	events: (): EventSource => {
		return new EventSource('/api/events', { withCredentials: true });
	},

	joinRequest: async (requestID: string): Promise<JoinRequestReview> => {
		const response = await apiFetch(`/api/agent/join-requests/${requestID}/review`);
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},

	approveJoinRequest: async (requestID: string): Promise<JoinApprovalResult> => {
		const response = await apiFetch(`/api/agent/join-requests/${requestID}/approve`, {
			method: 'POST',
		});
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},

	createJoinToken: async (): Promise<JoinToken> => {
		const response = await apiFetch('/api/agent/join-tokens', {
			method: 'POST',
			body: JSON.stringify({ node_name: '' }),
		});
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},

	joinTokens: async (): Promise<JoinTokenInfo[]> => {
		const response = await apiFetch('/api/agent/join-tokens');
		if (!response.ok) {
			throw new Error(await response.text());
		}
		return response.json();
	},

	revokeJoinToken: async (tokenID: string): Promise<void> => {
		const response = await apiFetch(`/api/agent/join-tokens/${tokenID}`, {
			method: 'DELETE',
		});
		if (!response.ok) {
			throw new Error(await response.text());
		}
	},
};
