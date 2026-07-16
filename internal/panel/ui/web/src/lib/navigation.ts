import { goto } from '$app/navigation';

export const backOrGoto = (fallback: string) => {
	if (typeof window !== 'undefined') {
		const stack = readHistoryStack();
		const current = `${window.location.pathname}${window.location.search}${window.location.hash}`;
		let target = stack.pop();

		while (target === current) {
			target = stack.pop();
		}

		sessionStorage.setItem('deft.history-stack', JSON.stringify(stack));
		if (target) {
			sessionStorage.setItem('deft.skip-next-history-push', 'true');
			goto(target);
			return;
		}
	}

	goto(fallback);
};

const readHistoryStack = () => {
	const rawValue = sessionStorage.getItem('deft.history-stack');
	if (!rawValue) return [];

	try {
		const value = JSON.parse(rawValue);
		return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : [];
	} catch {
		return [];
	}
};
