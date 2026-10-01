import { observer } from "mobx-react-lite";
import { useEffect, useState, type FormEvent } from "react";
import useSWR from "swr";

import { FormField } from "../components/form-field";
import { Alert } from "../components/ui/alert";
import { Button } from "../components/ui/button";
import { useT } from "../i18n/i18n";
import type { SlugAvailability, Workspace } from "../services/workspace.service";
import { useWorkspaces } from "../stores/context";
import { useForm } from "./form";
import type { FieldMessage } from "./problem-messages";
import { slugFrom, slugProblem, workspaceNameProblem } from "./slug";

/** How long the slug stays as typed before its availability is asked, in milliseconds. */
const settleMs = 300;

/** useSettled is value once it has stayed the same for ms. */
function useSettled<T>(value: T, ms: number): T {
  const [settled, setSettled] = useState(value);
  useEffect(() => {
    const timer = setTimeout(() => setSettled(value), ms);
    return () => clearTimeout(timer);
  }, [value, ms]);
  return settled;
}

/**
 * useAvailability is whether slug can name a new workspace, as the server
 * answers once the slug is spelled as one and has settled; undefined until
 * then. Each slug is asked once in a while: SWR keeps the answers of this
 * generation.
 */
function useAvailability(slug: string): SlugAvailability | undefined {
  const workspaces = useWorkspaces();
  const settled = useSettled(slug, settleMs);
  const asked = settled === slug && slugProblem(slug) === undefined ? slug : undefined;
  const { data } = useSWR(asked === undefined ? null : ["workspace-slug", asked], ([, s]: [string, string]) =>
    workspaces.checkSlug(s)
  );
  return asked === undefined ? undefined : data;
}

/** The message of a slug the server says cannot be used. */
const unavailable: Record<NonNullable<SlugAvailability["reason"]>, FieldMessage> = {
  taken: "field.slug.duplicate",
  reserved: "field.slug.not_allowed",
  invalid: "field.slug.invalid_format",
};

type CreateWorkspaceFormProps = {
  submitLabel: string;
  /** What the form does once the workspace is created; its failure shows as the creation's does. */
  onCreated: (workspace: Workspace) => Promise<void> | void;
  /** The button spans the form, as onboarding's steps have it. */
  wide?: boolean;
};

/**
 * CreateWorkspaceForm creates a workspace, of which the account becomes the
 * admin: the creation page and onboarding's workspace step (M2/P5 design
 * 3.5). The slug follows the name until the user types one; once it is
 * spelled as a slug and settles, the server says whether it is free. Its
 * answer only informs: the creation is what decides, a taken slug's 409
 * showing under the slug.
 */
export const CreateWorkspaceForm = observer(function CreateWorkspaceForm({
  submitLabel,
  onCreated,
  wide = false,
}: CreateWorkspaceFormProps) {
  const workspaces = useWorkspaces();
  const t = useT();
  const [name, setName] = useState("");
  const [typedSlug, setTypedSlug] = useState<string>();
  const slug = typedSlug ?? slugFrom(name);
  const availability = useAvailability(slug);
  const { ref, sending, banner, problemOf, submit } = useForm(["name", "slug"], { "workspace.slug_taken": "slug" });

  function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const found: Partial<Record<"name" | "slug", FieldMessage>> = {};
    const nameProblem = workspaceNameProblem(name);
    const problem = slugProblem(slug);
    if (nameProblem !== undefined) found.name = nameProblem;
    if (problem !== undefined) found.slug = problem;
    void submit(found, async () => {
      await onCreated(await workspaces.create({ name: name.trim(), slug }));
    });
  }

  const reason = availability?.available === false ? availability.reason : undefined;
  return (
    <form ref={ref} noValidate onSubmit={onSubmit} className="space-y-4">
      {banner !== undefined && <Alert>{banner}</Alert>}
      <FormField
        label={t("createWorkspace.name")}
        name="name"
        autoComplete="organization"
        value={name}
        error={problemOf("name")}
        onChange={(event) => setName(event.target.value)}
      />
      <FormField
        label={t("createWorkspace.slug")}
        name="slug"
        autoComplete="off"
        autoCapitalize="none"
        spellCheck={false}
        value={slug}
        error={problemOf("slug") ?? (reason === undefined ? undefined : t(unavailable[reason]))}
        hint={availability?.available === true ? t("createWorkspace.slugAvailable") : t("createWorkspace.slugHint")}
        onChange={(event) => setTypedSlug(event.target.value)}
      />
      <Button type="submit" className={wide ? "w-full" : undefined} disabled={sending}>
        {submitLabel}
      </Button>
    </form>
  );
});
