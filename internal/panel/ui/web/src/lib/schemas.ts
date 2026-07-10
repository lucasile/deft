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
});

export type CreateContainerForm = z.infer<typeof createContainerSchema>;
