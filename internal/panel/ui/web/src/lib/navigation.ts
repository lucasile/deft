import { goto } from '$app/navigation';

export const backOrGoto = (fallback: string) => {
	goto(fallback);
};
