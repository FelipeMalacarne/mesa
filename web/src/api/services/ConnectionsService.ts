/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { Connection } from '../models/Connection';
import type { CreateConnectionRequest } from '../models/CreateConnectionRequest';
import type { CreateDatabaseRequest } from '../models/CreateDatabaseRequest';
import type { CreateUserRequest } from '../models/CreateUserRequest';
import type { DBUser } from '../models/DBUser';
import type { ListDatabasesResponse } from '../models/ListDatabasesResponse';
import type { ListTablesResponse } from '../models/ListTablesResponse';
import type { OverviewResponse } from '../models/OverviewResponse';
import type { Session } from '../models/Session';
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
            errors: {
                404: `Not Found`,
            },
        });
    }
    /**
     * Get Server Health & Overview
     * @returns OverviewResponse Overview
     * @throws ApiError
     */
    public static getConnectionOverview({
        connectionId,
    }: {
        connectionId: string,
    }): CancelablePromise<OverviewResponse> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/connections/{connectionId}/overview',
            path: {
                'connectionId': connectionId,
            },
        });
    }
    /**
     * List databases from a connection
     * @returns ListDatabasesResponse Databases
     * @throws ApiError
     */
    public static listDatabases({
        connectionId,
    }: {
        connectionId: string,
    }): CancelablePromise<ListDatabasesResponse> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/connections/{connectionId}/databases',
            path: {
                'connectionId': connectionId,
            },
        });
    }
    /**
     * Create Database
     * @returns void
     * @throws ApiError
     */
    public static createDatabase({
        connectionId,
        requestBody,
    }: {
        connectionId: string,
        requestBody: CreateDatabaseRequest,
    }): CancelablePromise<void> {
        return __request(OpenAPI, {
            method: 'POST',
            url: '/connections/{connectionId}/databases',
            path: {
                'connectionId': connectionId,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * List tables from a database
     * @returns ListTablesResponse Tables
     * @throws ApiError
     */
    public static listTables({
        connectionId,
        databaseName,
    }: {
        connectionId: string,
        databaseName: string,
    }): CancelablePromise<ListTablesResponse> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/connections/{connectionId}/databases/{databaseName}/tables',
            path: {
                'connectionId': connectionId,
                'databaseName': databaseName,
            },
        });
    }
    /**
     * List database users (roles)
     * @returns DBUser Users
     * @throws ApiError
     */
    public static listUsers({
        connectionId,
    }: {
        connectionId: string,
    }): CancelablePromise<Array<DBUser>> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/connections/{connectionId}/users',
            path: {
                'connectionId': connectionId,
            },
        });
    }
    /**
     * Create a new database user
     * @returns void
     * @throws ApiError
     */
    public static createUser({
        connectionId,
        requestBody,
    }: {
        connectionId: string,
        requestBody: CreateUserRequest,
    }): CancelablePromise<void> {
        return __request(OpenAPI, {
            method: 'POST',
            url: '/connections/{connectionId}/users',
            path: {
                'connectionId': connectionId,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * List active sessions (Live Monitor)
     * @returns Session Sessions
     * @throws ApiError
     */
    public static listSessions({
        connectionId,
    }: {
        connectionId: string,
    }): CancelablePromise<Array<Session>> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/connections/{connectionId}/sessions',
            path: {
                'connectionId': connectionId,
            },
        });
    }
    /**
     * Kill a specific session
     * @returns void
     * @throws ApiError
     */
    public static killSession({
        connectionId,
        pid,
    }: {
        connectionId: string,
        pid: number,
    }): CancelablePromise<void> {
        return __request(OpenAPI, {
            method: 'DELETE',
            url: '/connections/{connectionId}/sessions/{pid}',
            path: {
                'connectionId': connectionId,
                'pid': pid,
            },
        });
    }
}
