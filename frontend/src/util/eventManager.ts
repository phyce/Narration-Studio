import { EventsOn, EventsEmit } from '../../wailsjs/runtime';

class EventManager {
	private static instance: EventManager;
	private subscriptions: { [key: string]: ((data?: any) => void)[] } = {};

	private constructor() {
        this.subscriptions = {};
    }

	public static getInstance(): EventManager {
		if (!EventManager.instance) {
			EventManager.instance = new EventManager();
		}
		return EventManager.instance;
	}

	subscribe(eventName: string, callback: (...data: any[]) => void): () => void {
        const unsubscribe = EventsOn(eventName, callback);

        if (!this.subscriptions[eventName]) {
            this.subscriptions[eventName] = [];
        }

        this.subscriptions[eventName].push(unsubscribe);

        return () => {
            const index = this.subscriptions[eventName].indexOf(unsubscribe);
            if (index !== -1) {
                unsubscribe();
                this.subscriptions[eventName].splice(index, 1);
            }

            if (this.subscriptions[eventName].length === 0) {
                delete this.subscriptions[eventName];
            }
        };
    }

    disableEvent(eventName: string): void {
        if (this.subscriptions[eventName]) {
            this.subscriptions[eventName].forEach(unsubscribe => unsubscribe());
            delete this.subscriptions[eventName];
        }
    }

	emit(eventName: string, data?: any): void {
		EventsEmit(eventName, data);
	}
}

export const eventManager = EventManager.getInstance();