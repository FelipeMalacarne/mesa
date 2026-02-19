/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { Connection } from '../models/Connection';
import type { CreateConnectionRequest } from '../models/CreateConnectionRequest';
import type { CreateDatabaseRequest } from '../models/CreateDatabaseRequest';
import type { CreateTableRequest } from '../models/CreateTableRequest';
import type { CreateUserRequest } from '../models/CreateUserRequest';
import type { Database } from '../models/Database';
import type { DBUser } from '../models/DBUser';
import type { OverviewResponse } from '../models/OverviewResponse';
import type { Session } from '../models/Session';
import type { Table } from '../models/Table';
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
            url: '/connections/{connectionID}',
            path: {
                'connectionID': connectionId,
            },
            errors: {
                404: `Not Found`,
            },
        });
    }
    /**
     * Checks the connection status
     * @returns void
     * @throws ApiError
     */
    public static pingConnection({
        connectionId,
    }: {
        /**
         * The unique identifier for the connection
         */
        connectionId: string,
    }): CancelablePromise<void> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/connections/{connectionID}/ping',
            path: {
                'connectionID': connectionId,
            },
            errors: {
                502: `Bad Gateway (Connection Failed)`,
            },
        });
    }
    /**
     * List databases from a connection
     * @returns Database Databases
     * @throws ApiError
     */
    public static listDatabases({
        connectionId,
    }: {
        /**
         * The unique identifier for the connection
         */
        connectionId: string,
    }): CancelablePromise<Array<Database>> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/connections/{connectionID}/databases',
            path: {
                'connectionID': connectionId,
            },
            errors: {
                500: `Internal Server Error`,
            },
        });
    }
    /**
     * Create a new database
     * @returns any Created
     * @throws ApiError
     */
    public static createDatabase({
        connectionId,
        requestBody,
    }: {
        /**
         * The unique identifier for the connection
         */
        connectionId: string,
        requestBody: CreateDatabaseRequest,
    }): CancelablePromise<any> {
        return __request(OpenAPI, {
            method: 'POST',
            url: '/connections/{connectionID}/databases',
            path: {
                'connectionID': connectionId,
            },
            body: requestBody,
            mediaType: 'application/json',
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
        /**
         * The unique identifier for the connection
         */
        connectionId: string,
    }): CancelablePromise<OverviewResponse> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/connections/{connectionID}/overview',
            path: {
                'connectionID': connectionId,
            },
        });
    }
    /**
     * List tables from a database
     * @returns Table Tables
     * @throws ApiError
     */
    public static listTables({
        connectionId,
        databaseName,
    }: {
        /**
         * The unique identifier for the connection
         */
        connectionId: string,
        /**
         * Database name
         */
        databaseName: string,
    }): CancelablePromise<Array<Table>> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/connections/{connectionID}/databases/{databaseName}/tables',
            path: {
                'connectionID': connectionId,
                'databaseName': databaseName,
            },
            errors: {
                500: `Internal Server Error`,
            },
        });
    }
    /**
     * Create a table in a database
     * @returns any Created
     * @throws ApiError
     */
    public static createTable({
        connectionId,
        databaseName,
        requestBody,
    }: {
        /**
         * The unique identifier for the connection
         */
        connectionId: string,
        /**
         * Database name
         */
        databaseName: string,
        requestBody: CreateTableRequest,
    }): CancelablePromise<any> {
        return __request(OpenAPI, {
            method: 'POST',
            url: '/connections/{connectionID}/databases/{databaseName}/tables',
            path: {
                'connectionID': connectionId,
                'databaseName': databaseName,
            },
            body: requestBody,
            mediaType: 'application/json',
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
        /**
         * The unique identifier for the connection
         */
        connectionId: string,
    }): CancelablePromise<Array<DBUser>> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/connections/{connectionID}/users',
            path: {
                'connectionID': connectionId,
            },
        });
    }
    /**
     * Create a new database user
     * @returns any Created
     * @throws ApiError
     */
    public static createUser({
        connectionId,
        requestBody,
    }: {
        /**
         * The unique identifier for the connection
         */
        connectionId: string,
        requestBody: CreateUserRequest,
    }): CancelablePromise<any> {
        return __request(OpenAPI, {
            method: 'POST',
            url: '/connections/{connectionID}/users',
            path: {
                'connectionID': connectionId,
            },
            body: requestBody,
            mediaType: 'application/json',
        });
    }
    /**
     * List active sessions
     * @returns Session Sessions
     * @throws ApiError
     */
    public static listSessions({
        connectionId,
    }: {
        /**
         * The unique identifier for the connection
         */
        connectionId: string,
    }): CancelablePromise<Array<Session>> {
        return __request(OpenAPI, {
            method: 'GET',
            url: '/connections/{connectionID}/sessions',
            path: {
                'connectionID': connectionId,
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
        /**
         * The unique identifier for the connection
         */
        connectionId: string,
        pid: number,
    }): CancelablePromise<void> {
        return __request(OpenAPI, {
            method: 'DELETE',
            url: '/connections/{connectionID}/sessions/{pid}',
            path: {
                'connectionID': connectionId,
                'pid': pid,
            },
        });
    }
}
