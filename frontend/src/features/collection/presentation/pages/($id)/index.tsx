import * as React from "react";
import {
  BookType,
  Box,
  Columns3,
  Database,
  Eye,
  MoreHorizontal,
  PlusSquareIcon,
  Search,
  SquarePen,
  Trash2,
} from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useNavigate, useRouter } from "@tanstack/react-router";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from "@/components/ui/input-group";
import {
  Page,
  PageActions,
  PageBody,
  PageHeader,
} from "@/components/ui/page";
import { aos } from "@/app/aos";
import { stores } from "@/app/lib/stores";
import { isDormant } from "@/lib/command-map";
import { DormantGate } from "@/components/DormantDomain";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { ButtonGroup } from "@/components/ui/button-group";
import {
  DataTable,
  DataTableProvider,
  useDataTable,
  DataTablePagination,
  DataTableToolbar,
  DataTableSearch,
  DataTableFilterMenu,
  DataTableExportMenu,
  DataTableViewOptions,
  generateFiltersFromData,
} from "@/components/ui/data-table";

const MotionSpan = motion.create("span");
const MotionTableRow = motion.create("tr");

function getRecordColumns(records: Record<string, unknown>[]) {
  const keys = new Set<string>();

  for (const record of records) {
    for (const key of Object.keys(record)) {
      if (key === "id" || key === "content" || key === "path") {
        continue;
      }

      keys.add(key);

      if (keys.size >= 8) {
        return Array.from(keys);
      }
    }
  }

  return Array.from(keys);
}

function renderCell(value: unknown) {
  if (value === null || value === undefined) {
    return "—";
  }

  if (typeof value === "string") {
    return value.length > 96 ? `${value.slice(0, 93)}...` : value;
  }

  return JSON.stringify(value);
}

function matchesRecordSearch(record: Record<string, unknown>, search: string) {
  if (!search.trim()) {
    return true;
  }

  const term = search.trim().toLowerCase();

  return Object.entries(record).some(([key, value]) => {
    if (key.toLowerCase().includes(term)) {
      return true;
    }

    if (typeof value === "string") {
      return value.toLowerCase().includes(term);
    }

    if (typeof value === "number" || typeof value === "boolean") {
      return String(value).toLowerCase().includes(term);
    }

    if (value && typeof value === "object") {
      return JSON.stringify(value).toLowerCase().includes(term);
    }

    return false;
  });
}

function AnimatedCount({ value, label }: { value: number; label: string }) {
  return (
    <span className="inline-flex items-center gap-1">
      <AnimatePresence initial={false} mode="popLayout">
        <MotionSpan
          key={value}
          initial={{ opacity: 0, y: 4, filter: "blur(4px)" }}
          animate={{ opacity: 1, y: 0, filter: "blur(0px)" }}
          exit={{ opacity: 0, y: -4, filter: "blur(4px)" }}
          transition={{ duration: 0.18, ease: "easeOut" }}
          className="inline-block tabular-nums"
        >
          {value}
        </MotionSpan>
      </AnimatePresence>
      <span>{label}</span>
    </span>
  );
}

export const CollectionPage = aos
  .page("/collections/$id")
  .withMetadata({
    title: "Collection",
    description: "Custom collection records",
  })
  .use(WorkspacePageMiddleware())
  .withLoader(async ({ client, request, response }) => {
    // Task 10: the `collection` domain is dormant — no Go backend to call
    // yet. Short-circuits before any client call so the dormant command's
    // empty envelope never reaches the `!collection.data` check below,
    // which would otherwise call `response.notFound()` and preempt
    // `DormantGate` (wrapping the returned JSX in `withComponent`) with
    // the 404 page instead.
    if (isDormant("collection")) {
      return { collection: undefined as any, records: [] as unknown[] };
    }

    try {

      const collection = await client.collection.getById.query({
        params: { collection: request.params.id },
      });

      if (!collection.data) {
        return response.notFound();
      }

      const records = await client.collection.listRecords.query({
        params: { collection: request.params.id },
        query: {},
      });

      // `collections_records-list` answers `{records, total}`
      // (`RecordsListOutput`, internal/domain/collection/commands.go), not
      // a bare array — `command-map.ts`'s own comment on `collection.
      // listRecords` already discloses this as a call-site fix. Reading
      // `records.data` directly handed the whole `{records, total}` object
      // to `allRecords` below wherever any record existed, breaking every
      // `.map`/`.length` use on it instead of quietly returning nothing.
      return {
        collection: collection.data.collection,
        records: records.data?.records ?? [],
      };
    } catch {
      return response.notFound();
    }
  })
  .withComponent(({ route, client }) => {
    const navigate = useNavigate();
    const router = useRouter();
    const collectionId = route.useParams().id;
    const loaderData = route.useLoaderData();

    const collection = loaderData.collection;
    const allRecords: Record<string, any>[] = loaderData.records;

    const { mutate: deleteRecord } =
      client.collection.deleteRecord.useMutation({
        onSuccess: async () => {
          toast.success("Record deleted.");
          await router.invalidate();
        },
        onError: (error) => {
          toast.error(
            error instanceof Error ? error.message : "Unable to delete record.",
          );
        },
      });

    const { mutate: deleteRecordAsync } =
      client.collection.deleteRecord.useMutation();

    const columns = React.useMemo(() => getRecordColumns(allRecords), [allRecords]);

    const tableColumns = React.useMemo(() => {
      const cols: any[] = [
        {
          accessorKey: "id",
          header: "id",
          cell: ({ row }: any) => {
            const recordId = row.original.id;
            return (
              <span className="font-mono text-xs font-medium text-foreground">
                {recordId}
              </span>
            );
          },
        },
      ];

      columns.forEach((colKey) => {
        cols.push({
          accessorKey: colKey,
          header: colKey,
          cell: ({ row }: any) => {
            return (
              <span className="text-muted-foreground truncate max-w-64 block">
                {renderCell(row.original[colKey])}
              </span>
            );
          },
        });
      });

      cols.push({
        id: "actions",
        header: "Actions",
        cell: ({ row }: any) => {
          const recordId = row.original.id;
          return (
            <ButtonGroup>
              <Button
                variant="outline"
                size="icon"
                onClick={() =>
                  navigate({
                    to: "/collections/$id/records/$record",
                    params: {
                      id: collectionId,
                      record: recordId,
                    },
                  })
                }
              >
                <SquarePen className="size-3" />
              </Button>
              <Button
                variant="outline"
                size="icon"
                onClick={() => {
                  if (!window.confirm(`Delete "${recordId}"?`)) {
                    return;
                  }
                  deleteRecord({
                    params: {
                      collection: collectionId,
                      record: recordId,
                    },
                  });
                }}
              >
                <Trash2 className="size-3" />
              </Button>
            </ButtonGroup>
          );
        },
      });

      return cols;
    }, [columns, collectionId, navigate, deleteRecord]);

    const filters = React.useMemo(() => {
      return generateFiltersFromData(allRecords, columns);
    }, [allRecords, columns]);

    return (
      <DormantGate feature="collection">
        <DataTableProvider
          data={allRecords}
          columns={tableColumns}
          filters={filters}
          enableRowSelection={true}
        >
          <CollectionPageContent
            collection={collection}
            collectionId={collectionId}
            allRecords={allRecords}
            deleteRecord={deleteRecord}
            deleteRecordAsync={deleteRecordAsync}
          />
        </DataTableProvider>
      </DormantGate>
    );
  })
  .build();

function CollectionPageContent({
  collection,
  collectionId,
  allRecords,
  deleteRecord,
  deleteRecordAsync,
}: {
  collection: any;
  collectionId: string;
  allRecords: any[];
  deleteRecord: any;
  deleteRecordAsync: any;
}) {
  const navigate = useNavigate();
  const router = useRouter();
  const { table } = useDataTable();
  const visibleCount = table.getFilteredRowModel().rows.length;
  const visibleColumnsCount = table.getVisibleFlatColumns().length;

  const selectedRows = table.getSelectedRowModel().flatRows;
  const hasSelection = selectedRows.length > 0;

  const handleDeleteSelected = async () => {
    if (selectedRows.length === 0) return;

    if (
      !window.confirm(
        `Are you sure you want to delete the ${selectedRows.length} selected record(s)?`
      )
    ) {
      return;
    }

    const deletePromises = selectedRows.map((row: any) =>
      deleteRecordAsync({
        params: {
          collection: collectionId,
          record: row.original.id,
        },
      })
    );

    toast.promise(Promise.all(deletePromises), {
      loading: `Deleting ${selectedRows.length} record(s)...`,
      success: () => {
        table.resetRowSelection();
        router.invalidate();
        return `${selectedRows.length} record(s) deleted successfully.`;
      },
      error: (err) => {
        return err instanceof Error ? err.message : "Failed to delete some records.";
      },
    });
  };

  return (
    <Page>
      <PageHeader>
        <div className="flex min-w-0 items-center gap-3">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h1 className="truncate text-sm font-semibold text-foreground">
                {collection.name}
              </h1>
              <ButtonGroup className="bg-secondary/30 rounded-full">
                <Badge variant="outline">
                  <BookType
                    data-icon="inline-start"
                    className="size-3 text-muted-foreground"
                  />
                  {collection.format}
                </Badge>
                <Badge variant="outline">
                  <Box
                    data-icon="inline-start"
                    className="size-3 text-muted-foreground"
                  />
                  {collection.scope === "skill"
                    ? `skill:${collection.skill}`
                    : "workspace"}
                </Badge>
                <Badge variant="outline">
                  <Database
                    data-icon="inline-start"
                    className="size-3 text-muted-foreground"
                  />
                  <AnimatedCount value={allRecords.length} label="total" />
                </Badge>
                <Badge variant="outline">
                  <Eye
                    data-icon="inline-start"
                    className="size-3 text-muted-foreground"
                  />
                  <AnimatedCount value={visibleCount} label="visible" />
                </Badge>
                <Badge variant="outline">
                  <Columns3
                    data-icon="inline-start"
                    className="size-3 text-muted-foreground"
                  />
                  <AnimatedCount value={visibleColumnsCount} label="columns" />
                </Badge>
              </ButtonGroup>
            </div>
          </div>
        </div>
        <PageActions>
          <DataTableSearch
            placeholder="Search records..."
            className="h-8 w-full max-w-[160px] lg:max-w-[240px] bg-background border rounded-lg"
          />
          <DataTableFilterMenu size="sm" variant="ghost" />
          <ButtonGroup>
            <DataTableViewOptions />
            <DataTableExportMenu />
          </ButtonGroup>
          {hasSelection && (
            <Button
              variant="destructive"
              size="sm"
              className="h-8"
              onClick={handleDeleteSelected}
            >
              <Trash2 className="size-4 mr-2" />
              Delete Selected ({selectedRows.length})
            </Button>
          )}
          <Button
            onClick={() =>
              void navigate({
                to: "/collections/$id/records/$record",
                params: { id: collectionId, record: "new" },
              })
            }
          >
            <PlusSquareIcon />
            Create
          </Button>
        </PageActions>
      </PageHeader>

      <PageBody className="min-h-0 gap-0 overflow-hidden p-0 flex flex-col">
        <div className="flex-1 overflow-auto max-h-[calc(100vh-20rem)]">
          <DataTable className="w-full text-sm" />
        </div>

        <DataTablePagination />
      </PageBody>
    </Page>
  );
}
