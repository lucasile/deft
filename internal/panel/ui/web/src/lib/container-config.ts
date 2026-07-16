import type { CreateContainerConfig } from '$lib/api/client';

export type CreateContainerFormData = {
	ports: string;
	env: string;
	volumes: string;
	restart_policy: 'no' | 'always' | 'unless-stopped' | 'on-failure';
};

export const parseCreateConfig = (data: CreateContainerFormData): CreateContainerConfig => {
	const config: CreateContainerConfig = {
		restart_policy: data.restart_policy,
	};

	const ports = parsePorts(data.ports);
	if (ports.length > 0) config.ports = ports;

	const env = parseEnv(data.env);
	if (env.length > 0) config.env = env;

	const volumes = parseVolumes(data.volumes);
	if (volumes.length > 0) config.volumes = volumes;

	return config;
};

const parsePorts = (value: string) => {
	return lines(value).map((line) => {
		const [left, protocolValue = 'tcp'] = line.split('/');
		const [hostValue, containerValue] = left.split(':');
		const hostPort = Number(hostValue);
		const containerPort = Number(containerValue);
		const protocol = protocolValue.trim().toLowerCase();
		if (!Number.isInteger(hostPort) || !Number.isInteger(containerPort) || !['tcp', 'udp'].includes(protocol)) {
			throw new Error('Ports must use host:container/protocol, for example 25565:25565/tcp');
		}
		return { host_port: hostPort, container_port: containerPort, protocol: protocol as 'tcp' | 'udp' };
	});
};

const parseEnv = (value: string) => {
	return lines(value).map((line) => {
		const separator = line.indexOf('=');
		if (separator <= 0) {
			throw new Error('Environment variables must use KEY=value');
		}
		return { key: line.slice(0, separator).trim(), value: line.slice(separator + 1) };
	});
};

const parseVolumes = (value: string) => {
	return lines(value).map((line) => {
		const parts = line.split(':');
		if (parts.length < 2 || parts.length > 3) {
			throw new Error('Volumes must use host:container or host:container:ro');
		}
		const [hostPath, containerPath, mode = 'rw'] = parts;
		if (!['rw', 'ro'].includes(mode)) {
			throw new Error('Volume mode must be rw or ro');
		}
		return { host_path: hostPath.trim(), container_path: containerPath.trim(), read_only: mode === 'ro' };
	});
};

const lines = (value: string) =>
	value
		.split('\n')
		.map((line) => line.trim())
		.filter(Boolean);
