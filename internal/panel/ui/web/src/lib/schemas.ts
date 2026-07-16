import { z } from 'zod';

export const createContainerSchema = z.object({
	name: z
		.string()
		.min(1, 'Name is required')
		.max(64, 'Name must be 64 characters or less')
		.regex(/^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$/, 'Use letters, numbers, dots, underscores, or dashes'),
	image: z
		.string()
		.min(1, 'Image is required')
		.max(255, 'Image must be 255 characters or less')
		.refine((value) => !value.includes('://') && !/\s/.test(value), 'Enter a Docker image reference'),
	ports: z.string().max(2000, 'Ports must be 2000 characters or less').default(''),
	env: z.string().max(8000, 'Environment must be 8000 characters or less').default(''),
	volumes: z.string().max(4000, 'Volumes must be 4000 characters or less').default(''),
	restart_policy: z.enum(['no', 'always', 'unless-stopped', 'on-failure']).default('no'),
});

export type CreateContainerForm = z.infer<typeof createContainerSchema>;
