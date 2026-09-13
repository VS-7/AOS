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
import { useAlert } from "@/components/ui/alert-provider";
import { WorkspacePageMiddleware } from "@/features/workspace/presentation/middlewares/workspace.middleware";
import { ButtonGroup } from "@/components/ui/button-group";
import { t } from "@/lib/i18n";
import { errorMessage } from "@/lib/aos-facade";
import { formatViewValue } from "@/lib/view-spec";
import {
  fieldsOf,
  recordLabel,
  type CollectionDefinition,
  type CollectionField,
  type CollectionRecord,
} from "../../helpers/collection-fields.helper";
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

/**
 * One table row: the record's declared fields, each already in the form a
 * person reads, beside the record itself.
 *
 * The table used to be built from the keys of the record *envelope* — `id`,
 * `collection`, `data`, `createdAt`, `updatedAt` — so its columns were
 * "Collection | Data | Created At", the Data cell raw JSON and both dates
 * `0001-01-01` (a record has no timestamps unless its collection declares
 * them). Search and filters only ever saw that JSON.
 *
 * Cells are display strings so the search, the filters and the export all
 * work on what is on screen. The id and the record ride under keys a field
 * name cannot take (field names are identifiers, never `__`-prefixed here).
 */
type RecordRow = Record<string, string> & { __id: string; __record: CollectionRecord };

function toRows(fields: CollectionField[], records: CollectionRecord[]): RecordRow[] {
  return records.map((record) => {
    const row: Record<string, unknown> = { __id: record.id, __record: record };
    for (const field of fields) {
      row[field.name] = formatViewValue(record.data?.[field.name]);
    }
    return row as RecordRow;
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
      return {
        collection: undefined as unknown as CollectionDefinition,
        records: [] as CollectionRecord[],
        recordsError: null as string | null,
      };
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
        collection: collection.data.collection as CollectionDefinition,
        records: (records.data?.records ?? []) as CollectionRecord[],
        // A refused list is not an empty collection. It used to render as
        // "0 total — Nothing Here!" over a collection that had records.
        recordsError: records.error ? (errorMessage(records.error) ?? t("The records could not be listed.")) : null,
      };
    } catch {
      return response.notFound();
    }
  })
  .withComponent(({ route, client }) => {
    const navigate = useNavigate();
    const router = useRouter();
    const { confirm } = useAlert();
    const collectionId = route.useParams().id;
    const loaderData = route.useLoaderData();

    const collection = loaderData.collection;
    const fields = React.useMemo(() => fieldsOf(collection), [collection]);
    const allRecords = React.useMemo(
      () => toRows(fields, loaderData.records),
      [fields, loaderData.records],
    );

    const { mutate: deleteRecord } =
      client.collection.deleteRecord.useMutation({
        onSuccess: async () => {
          toast.success(t("Record deleted."));
          await router.invalidate();
        },
        onError: (error) => {
          toast.error(errorMessage(error) ?? t("Unable to delete record."));
        },
      });

    const columns = React.useMemo(() => fields.map((field) => field.name), [fields]);

    const tableColumns = React.useMemo(() => {
      const cols: any[] = [
        {
          accessorKey: "__id",
          header: "id",
          cell: ({ row }: any) => {
            const recordId = row.original.__id;
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
                {row.original[colKey] || "—"}
              </span>
            );
          },
        });
      });

      cols.push({
        id: "actions",
        header: () => t("Actions"),
        cell: ({ row }: any) => {
          const recordId = row.original.__id;
          const label = recordLabel(fields, row.original.__record?.data) ?? recordId;
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
                onClick={async () => {
                  // Not `window.confirm`: WKWebView routes it to a delegate
                  // method Wails does not implement, so it answered false
                  // without asking and records could not be deleted from the
                  // desktop window. See lib/wails.ts.
                  const accepted = await confirm({
                    title: t("Delete \"{{name}}\"?", { name: label }),
                    description: t("This record cannot be recovered."),
                    confirmText: t("Delete"),
                    variant: "destructive",
                  });
                  if (!accepted) return;

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
    }, [columns, fields, collectionId, navigate, deleteRecord, confirm]);

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
            recordsError={loaderData.recordsError}
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
  recordsError,
}: {
  collection: CollectionDefinition;
  collectionId: string;
  allRecords: RecordRow[];
  recordsError: string | null;
}) {
  const navigate = useNavigate();
  const router = useRouter();
  const { confirm } = useAlert();
  const { table } = useDataTable();
  const visibleCount = table.getFilteredRowModel().rows.length;
  const visibleColumnsCount = table.getVisibleFlatColumns().length;

  const selectedRows = table.getSelectedRowModel().flatRows;
  const hasSelection = selectedRows.length > 0;

  const handleDeleteSelected = async () => {
    if (selectedRows.length === 0) return;

    // See the per-row delete above on why this is not `window.confirm`.
    const accepted = await confirm({
      title: t("Delete {{count}} record(s)?", { count: selectedRows.length }),
      description: t("The selected records cannot be recovered."),
      confirmText: t("Delete"),
      variant: "destructive",
    });
    if (!accepted) return;

    // `mutateOrThrow`, not the hook's `mutate`: React Query's `mutate`
    // returns nothing, so this used to hand `Promise.all` an array of
    // `undefined` that resolved at once — "N record(s) deleted successfully."
    // before a single delete had even been answered, refused or not.
    const deletions = selectedRows.map((row: any) =>
      aos.client.collection.deleteRecord.mutateOrThrow({
        params: {
          collection: collectionId,
          record: row.original.__id,
        },
      })
    );

    toast.promise(
      Promise.allSettled(deletions).then((results) => {
        // Some of them may have landed: the table has to show that either way.
        router.invalidate();
        const refused = results.filter(
          (result): result is PromiseRejectedResult => result.status === "rejected",
        );
        if (refused.length > 0) {
          throw new Error(
            t("{{failed}} of {{count}} record(s) could not be deleted: {{reason}}", {
              failed: refused.length,
              count: results.length,
              reason: errorMessage(refused[0]!.reason) ?? "",
            }),
          );
        }
        table.resetRowSelection();
      }),
      {
        loading: t("Deleting {{count}} record(s)...", { count: selectedRows.length }),
        success: () => t("{{count}} record(s) deleted.", { count: selectedRows.length }),
        error: (err) => errorMessage(err) ?? t("Failed to delete some records."),
      },
    );
  };

  return (
    <Page>
      <PageHeader>
        <div className="flex min-w-0 items-center gap-3">
          <div className="min-w-0">
            <div className="flex min-w-0 items-center gap-2">
              {/* The name keeps its room; the badges beside it give way first.
                  It used to shrink to "Con…" the moment the toolbar grew. */}
              <h1 className="max-w-[20rem] shrink-0 truncate text-sm font-semibold text-foreground">
                {collection.name}
              </h1>
              <ButtonGroup className="bg-secondary/30 min-w-0 overflow-hidden rounded-full">
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
                    : t("workspace")}
                </Badge>
                <Badge variant="outline">
                  <Database
                    data-icon="inline-start"
                    className="size-3 text-muted-foreground"
                  />
                  <AnimatedCount value={allRecords.length} label={t("total")} />
                </Badge>
                <Badge variant="outline">
                  <Eye
                    data-icon="inline-start"
                    className="size-3 text-muted-foreground"
                  />
                  <AnimatedCount value={visibleCount} label={t("visible")} />
                </Badge>
                <Badge variant="outline">
                  <Columns3
                    data-icon="inline-start"
                    className="size-3 text-muted-foreground"
                  />
                  <AnimatedCount value={visibleColumnsCount} label={t("columns")} />
                </Badge>
              </ButtonGroup>
            </div>
          </div>
        </div>
        <PageActions>
          <DataTableSearch
            placeholder={t("Search records...")}
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
              {t("Delete selected ({{count}})", { count: selectedRows.length })}
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
            {t("Create")}
          </Button>
        </PageActions>
      </PageHeader>

      <PageBody className="min-h-0 gap-0 overflow-hidden p-0 flex flex-col">
        {recordsError ? (
          <div role="alert" className="m-6 rounded-md border border-destructive/40 bg-destructive/5 px-4 py-3 text-sm text-destructive">
            {t("The records of this collection could not be listed: {{reason}}", { reason: recordsError })}
          </div>
        ) : (
          <>
            <div className="flex-1 overflow-auto max-h-[calc(100vh-20rem)]">
              <DataTable className="w-full text-sm" />
            </div>

            <DataTablePagination />
          </>
        )}
      </PageBody>
    </Page>
  );
}
