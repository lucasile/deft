import type { Server } from '$lib/api/client';

export type ServerState =
	| 'creating'
	| 'starting'
	| 'online'
	| 'stopping'
	| 'offline'
	| 'restarting'
	| 'removing'
	| 'failed'
	| 'missing'
	| 'unknown';

export type BadgeVariant = 'default' | 'secondary' | 'success' | 'warning' | 'destructive';

export const serverState = (status = ''): ServerState => {
	switch (status) {
		case 'create_requested':
		case 'creating':
			return 'creating';
		case 'start_requested':
		case 'starting':
			return 'starting';
		case 'running':
		case 'online':
			return 'online';
		case 'stop_requested':
		case 'stopping':
			return 'stopping';
		case 'restart_requested':
		case 'restarting':
			return 'restarting';
		case 'remove_requested':
		case 'removing':
			return 'removing';
		case 'created':
		case 'exited':
		case 'stopped':
		case 'offline':
			return 'offline';
		case 'failed':
			return 'failed';
		case 'missing':
			return 'missing';
		default:
			return status ? 'unknown' : 'unknown';
	}
};

export const serverStateLabel = (status = '') => {
	const state = serverState(status);
	if (state === 'online') return 'online';
	if (state === 'offline') return 'offline';
	return state;
};

export const serverStateVariant = (status = ''): BadgeVariant => {
	const state = serverState(status);
	if (state === 'online') return 'success';
	if (state === 'creating' || state === 'starting' || state === 'stopping' || state === 'restarting' || state === 'removing') return 'warning';
	if (state === 'failed' || state === 'missing') return 'destructive';
	return 'default';
};

export const serverStateBusy = (status = '') => {
	const state = serverState(status);
	return state === 'creating' || state === 'starting' || state === 'stopping' || state === 'restarting' || state === 'removing';
};

export const serverCanStart = (server: Server | null, status = server?.status || '') => {
	return Boolean(server?.container_id && serverState(status) === 'offline');
};

export const serverCanStop = (server: Server | null, status = server?.status || '') => {
	return Boolean(server?.container_id && serverState(status) === 'online');
};

export const serverCanRestart = (server: Server | null, status = server?.status || '') => {
	return Boolean(server?.container_id && serverState(status) === 'online');
};

export const serverCanRemove = (server: Server | null, status = server?.status || '') => {
	return Boolean(server?.container_id && !serverStateBusy(status));
};
