<script lang="ts">
	import { afterNavigate } from '$app/navigation';
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';

	let { children } = $props();

	afterNavigate(({ from, to }) => {
		if (!from || !to || from.url.origin !== to.url.origin) return;
		if (sessionStorage.getItem('deft.skip-next-history-push') === 'true') {
			sessionStorage.removeItem('deft.skip-next-history-push');
			return;
		}

		const fromPath = `${from.url.pathname}${from.url.search}${from.url.hash}`;
		const toPath = `${to.url.pathname}${to.url.search}${to.url.hash}`;
		if (fromPath === toPath) return;

		const stack = readHistoryStack();
		if (stack.at(-1) !== fromPath) {
			stack.push(fromPath);
		}
		sessionStorage.setItem('deft.history-stack', JSON.stringify(stack.slice(-25)));
	});

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
</script>

<svelte:head><link rel="icon" href={favicon} /></svelte:head>
{@render children()}
