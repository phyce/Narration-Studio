import type {ObjectDirective} from 'vue';
import Tooltip from 'primevue/tooltip';

type TooltipPosition = 'top' | 'bottom' | 'left' | 'right';

type TooltipHost = HTMLElement & {$_ptooltipModifiers: Partial<Record<TooltipPosition, boolean>>};

type TooltipInstance = {
	alignTop(host: TooltipHost): void;
	alignBottom(host: TooltipHost): void;
	alignLeft(host: TooltipHost): void;
	alignRight(host: TooltipHost): void;
	isOutOfBounds(host: TooltipHost): boolean;
	getTooltipElement(host: TooltipHost): HTMLElement;
	alignInsideWindow(host: TooltipHost, position: TooltipPosition): void;
};

const windowMargin = 8;

const positionsToTry: Record<TooltipPosition, TooltipPosition[]> = {
	top: ['top', 'bottom'],
	bottom: ['bottom', 'top'],
	left: ['left', 'right', 'top', 'bottom'],
	right: ['right', 'left', 'top', 'bottom'],
};

const extendable = Tooltip as ObjectDirective & {extend(name: string, options: object): ObjectDirective};

// PrimeVue flips a tooltip to the other side when it doesn't fit, but never slides it along,
// so a tooltip centred on an icon near the window's edge was cut off. Its fit-content width
// also shrank a tooltip near the right edge instead of letting it move.
export default extendable.extend('tooltip', {
	methods: {
		align(this: TooltipInstance, host: TooltipHost) {
			this.getTooltipElement(host).style.width = 'max-content';
			const modifiers = host.$_ptooltipModifiers;
			const preferred = (['top', 'bottom', 'left'] as const).find(position => modifiers[position]) ?? 'right';
			for (const position of positionsToTry[preferred]) {
				this.alignInsideWindow(host, position);
				if (!this.isOutOfBounds(host)) {
					return;
				}
			}
			this.alignInsideWindow(host, preferred);
		},
		alignInsideWindow(this: TooltipInstance, host: TooltipHost, position: TooltipPosition) {
			const alignments = {top: this.alignTop, bottom: this.alignBottom, left: this.alignLeft, right: this.alignRight};
			alignments[position].call(this, host);

			const tooltip = this.getTooltipElement(host);
			const arrow = tooltip.querySelector<HTMLElement>('[data-pc-section="arrow"]');
			const tooltipBounds = tooltip.getBoundingClientRect();
			const hostBounds = host.getBoundingClientRect();
			if (position === 'top' || position === 'bottom') {
				const shift = shiftIntoWindow(tooltipBounds.left, tooltipBounds.width, document.documentElement.clientWidth);
				tooltip.style.left = `${parseFloat(tooltip.style.left) + shift}px`;
				if (arrow) {
					arrow.style.left = `${hostBounds.left + hostBounds.width / 2 - tooltipBounds.left - shift}px`;
				}
			} else {
				const shift = shiftIntoWindow(tooltipBounds.top, tooltipBounds.height, document.documentElement.clientHeight);
				tooltip.style.top = `${parseFloat(tooltip.style.top) + shift}px`;
				if (arrow) {
					arrow.style.top = `${hostBounds.top + hostBounds.height / 2 - tooltipBounds.top - shift}px`;
				}
			}
		},
	},
});

function shiftIntoWindow(start: number, size: number, windowSize: number): number {
	const lowest = windowMargin;
	const highest = Math.max(lowest, windowSize - windowMargin - size);
	return Math.min(Math.max(start, lowest), highest) - start;
}
