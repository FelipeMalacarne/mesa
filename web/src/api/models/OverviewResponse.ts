/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
export type OverviewResponse = {
    status: OverviewResponse.status;
    version?: string;
    uptime?: string;
    sessions?: string;
    latency_ms: number;
};
export namespace OverviewResponse {
    export enum status {
        ONLINE = 'ONLINE',
        UNREACHABLE = 'UNREACHABLE',
    }
}

