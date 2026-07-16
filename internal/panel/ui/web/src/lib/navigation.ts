import { goto } from '$app/navigation';

export const backOrGoto = (fallback: string) => {
	if (typeof window !== 'undefined' && document.referrer) {
		try {
			const referrer = new URL(document.referrer);
			if (referrer.origin === window.location.origin && window.history.length > 1) {
				window.history.back();
				return;
			}
		} catch {
			// Fall through to the explicit route when the referrer is not parseable.
		}
	}

	goto(fallback);
};
