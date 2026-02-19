/* generated using openapi-typescript-codegen -- do not edit */
/* istanbul ignore file */
/* tslint:disable */
/* eslint-disable */
import type { ColumnDataType } from './ColumnDataType';
export type CreateTableColumn = {
    name: string;
    type: ColumnDataType;
    length?: number;
    precision?: number;
    nullable?: boolean;
    primary_key?: boolean;
    default_value?: string;
};

