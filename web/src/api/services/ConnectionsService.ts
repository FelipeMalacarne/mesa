/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { Connection } from '../models/Connection';
import type { CreateConnectionRequest } from '../models/CreateConnectionRequest';
import type { CancelablePromise } from '../core/CancelablePromise';
import { OpenAPI } from '../core/OpenAPI';
import { request as __request } from '../core/request';
export class ConnectionsService {
    /**
     * List Connections
     * @returns Connection Connections
     * @throws ApiError
     */
    public static listConnections(): CancelablePromise<Array<Connection>> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/connections',
        });
    }
    /**
     * Create Connection
     * @returns Connection Created
     * @throws ApiError
     */
    public static createConnection({
        requestBody,
    }: {
        requestBody: CreateConnectionRequest,
    }): CancelablePromise<Connection> {
        return __request(OpenAPI, {
            method: 'POST',
            url: '/connections',
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * Retrieves a connection by ID
     * @returns Connection Connection
     * @throws ApiError
     */
    public static findConnection({
        connectionId,
    }: {
        /**
         * The unique identifier for the connection
         */
        connectionId: string,
    }): CancelablePromise<Connection> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/connections/{connectionId}',
            path: {
                'connectionId': connectionId,
            },
        });
    }
}
