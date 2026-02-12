/**
 * Datadog RUM Integration for Rayne Frontend
 *
 * This script:
 * 1. Loads Datadog Browser RUM SDK from CDN
 * 2. Calls backend POST /v1/rum/init to get visitor UUID
 * 3. Sets UUID as Datadog user identity via DD_RUM.setUser()
 * 4. Stores UUID in localStorage for persistence
 * 5. Tracks page views to backend
 * 6. Handles session end on page unload
 */

(function() {
    'use strict';

    const STORAGE_KEYS = {
        VISITOR_UUID: 'rayne_visitor_uuid',
        SESSION_ID: 'rayne_session_id',
        SESSION_START: 'rayne_session_start'
    };

    const RUM_CONFIG = {
        applicationId: '6d730a61-be91-4cec-80fb-80848bb29d14',
        clientToken: 'pub902cdeb5b6dd38e7179c22ec46cf6112',
        site: 'datadoghq.com',
        service: 'rayne-frontend',
        env: 'staging',
        version: '2.1.0',
        sessionSampleRate: 100,
        sessionReplaySampleRate: 20,
        trackUserInteractions: true,
        trackResources: true,
        trackLongTasks: true,
        defaultPrivacyLevel: 'mask-user-input',
        traceContextInjection: 'all',
        compressIntakeRequests: true,
        allowedTracingUrls: [
            { match: /localhost/, propagatorTypes: ['datadog'] },
            { match: /rayne/, propagatorTypes: ['datadog'] },
            { match: /n0kos\.com/, propagatorTypes: ['datadog'] }
        ]
    };


    function getApiBase() {
        return window.RAYNE_API_BASE || 'http://localhost:8080';
    }

    function loadDatadogSDK() {
        return new Promise((resolve, reject) => {
            if (window.DD_RUM) {
                resolve(window.DD_RUM);
                return;
            }

            const script = document.createElement('script');
            script.src = 'https://www.datadoghq-browser-agent.com/us1/v6/datadog-rum.js';
            script.async = true;
            script.onload = () => {
                if (window.DD_RUM) {
                    resolve(window.DD_RUM);
                } else {
                    reject(new Error('DD_RUM not available after script load'));
                }
            };
            script.onerror = () => reject(new Error('Failed to load Datadog RUM SDK'));
            document.head.appendChild(script);
        });
    }

    async function initVisitor() {
        const existingUuid = localStorage.getItem(STORAGE_KEYS.VISITOR_UUID);
        const existingSessionId = localStorage.getItem(STORAGE_KEYS.SESSION_ID);

        try {
            const response = await fetch(`${getApiBase()}/v1/rum/init`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    visitor_uuid: existingUuid || undefined,
                    session_id: existingSessionId || undefined,
                    user_agent: navigator.userAgent,
                    referrer: document.referrer || null,
                    page_url: window.location.href
                })
            });

            if (!response.ok) {
                throw new Error(`Backend returned ${response.status}`);
            }

            const data = await response.json();

            localStorage.setItem(STORAGE_KEYS.VISITOR_UUID, data.visitor_uuid);
            localStorage.setItem(STORAGE_KEYS.SESSION_ID, data.session_id);
            localStorage.setItem(STORAGE_KEYS.SESSION_START, Date.now().toString());

            return data;
        } catch (error) {
            console.warn('Failed to init visitor with backend, using local UUID:', error);
            const localUuid = existingUuid || crypto.randomUUID();
            const localSessionId = existingSessionId || crypto.randomUUID();

            localStorage.setItem(STORAGE_KEYS.VISITOR_UUID, localUuid);
            localStorage.setItem(STORAGE_KEYS.SESSION_ID, localSessionId);
            localStorage.setItem(STORAGE_KEYS.SESSION_START, Date.now().toString());

            return {
                visitor_uuid: localUuid,
                session_id: localSessionId
            };
        }
    }

    async function trackPageView(visitorData) {
        try {
            await fetch(`${getApiBase()}/v1/rum/track`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    visitor_uuid: visitorData.visitor_uuid,
                    session_id: visitorData.session_id,
                    event_type: 'page_view',
                    page_url: window.location.href,
                    page_title: document.title,
                    referrer: document.referrer || null,
                    timestamp: new Date().toISOString()
                })
            });
        } catch (error) {
            console.warn('Failed to track page view:', error);
        }
    }

    function setupSessionEndTracking(visitorData) {
        const endSession = () => {
            const sessionStart = localStorage.getItem(STORAGE_KEYS.SESSION_START);
            const duration = sessionStart ? Date.now() - parseInt(sessionStart, 10) : 0;

            navigator.sendBeacon(
                `${getApiBase()}/v1/rum/session/end`,
                JSON.stringify({
                    visitor_uuid: visitorData.visitor_uuid,
                    session_id: visitorData.session_id,
                    duration_ms: duration,
                    page_count: window.performance?.navigation?.redirectCount || 1
                })
            );
        };

        window.addEventListener('beforeunload', endSession);
        document.addEventListener('visibilitychange', () => {
            if (document.visibilityState === 'hidden') {
                endSession();
            }
        });
    }

    async function init() {
        try {
            const [DD_RUM, visitorData] = await Promise.all([
                loadDatadogSDK(),
                initVisitor()
            ]);

            DD_RUM.init(RUM_CONFIG);

            DD_RUM.setUser({
                id: visitorData.visitor_uuid,
                session_id: visitorData.session_id,
                name: `Visitor ${visitorData.visitor_uuid.substring(0, 8)}`
            });

            await trackPageView(visitorData);

            setupSessionEndTracking(visitorData);

            console.log('Datadog RUM initialized with visitor:', visitorData.visitor_uuid);

            window.rayneRUM = {
                visitorUuid: visitorData.visitor_uuid,
                sessionId: visitorData.session_id,
                trackEvent: async (eventType, metadata = {}) => {
                    try {
                        await fetch(`${getApiBase()}/v1/rum/track`, {
                            method: 'POST',
                            headers: { 'Content-Type': 'application/json' },
                            body: JSON.stringify({
                                visitor_uuid: visitorData.visitor_uuid,
                                session_id: visitorData.session_id,
                                event_type: eventType,
                                page_url: window.location.href,
                                metadata: metadata,
                                timestamp: new Date().toISOString()
                            })
                        });

                        DD_RUM.addAction(eventType, metadata);
                    } catch (error) {
                        console.warn('Failed to track event:', error);
                    }
                }
            };

        } catch (error) {
            console.error('Failed to initialize Datadog RUM:', error);
        }
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
