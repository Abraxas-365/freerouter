import { useEffect, useState } from "react"
import {
  Plus, Trash2, Users as UsersIcon, ShieldCheck, Link2, UserX, Pencil,
} from "lucide-react"
import { toast } from "sonner"
import { useApi } from "@/api"
import type {
  AccessUser, CreateUserRequest, UpdateUserRequest,
  AccessRole, CreateRoleRequest, UpdateRoleRequest,
  RoleAssignment, AssignRoleRequest,
} from "@/api/types"
import { VALID_PERMISSIONS } from "@/api/types"
import { PageHeader, EmptyState, ConfirmDialog } from "@/components"
import { MetricCardSkeleton } from "@/components/feedback/skeletons"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import {
  Dialog, DialogContent, DialogDescription, DialogFooter,
  DialogHeader, DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"

// =============================================================================
// Main Page
// =============================================================================

export default function AccessPage() {
  const api = useApi()
  const [users, setUsers] = useState<AccessUser[]>([])
  const [roles, setRoles] = useState<AccessRole[]>([])
  const [assignments, setAssignments] = useState<RoleAssignment[]>([])
  const [loading, setLoading] = useState(true)

  // dialog state
  const [createUserOpen, setCreateUserOpen] = useState(false)
  const [editUser, setEditUser] = useState<AccessUser | null>(null)
  const [suspendUserId, setSuspendUserId] = useState<string | null>(null)
  const [createRoleOpen, setCreateRoleOpen] = useState(false)
  const [editRole, setEditRole] = useState<AccessRole | null>(null)
  const [deleteRoleId, setDeleteRoleId] = useState<string | null>(null)
  const [assignOpen, setAssignOpen] = useState(false)
  const [unassign, setUnassign] = useState<AssignRoleRequest | null>(null)

  async function load() {
    const [u, r, a] = await Promise.all([
      api.access.listUsers(),
      api.access.listRoles(),
      api.access.listAssignments(),
    ])
    setUsers(u)
    setRoles(r)
    setAssignments(a)
    setLoading(false)
  }

  useEffect(() => { load() }, [api])

  if (loading) {
    return (
      <div className="space-y-6 p-6">
        <PageHeader title="Access Management" description="Manage users, roles, and permissions" />
        <MetricCardSkeleton count={3} />
      </div>
    )
  }

  return (
    <div className="space-y-6 p-6">
      <PageHeader title="Access Management" description="Manage users, roles, and permissions" />

      <Tabs defaultValue="users">
        <TabsList>
          <TabsTrigger value="users" className="gap-1.5">
            <UsersIcon className="h-3.5 w-3.5" /> Users
          </TabsTrigger>
          <TabsTrigger value="roles" className="gap-1.5">
            <ShieldCheck className="h-3.5 w-3.5" /> Roles
          </TabsTrigger>
          <TabsTrigger value="assignments" className="gap-1.5">
            <Link2 className="h-3.5 w-3.5" /> Assignments
          </TabsTrigger>
        </TabsList>

        {/* ── Users Tab ────────────────────────────────────────────── */}
        <TabsContent value="users" className="space-y-4">
          <div className="flex justify-end">
            <Button size="sm" onClick={() => setCreateUserOpen(true)}>
              <Plus className="h-4 w-4 mr-1" /> Create User
            </Button>
          </div>
          {users.length === 0 ? (
            <EmptyState
              icon={UsersIcon}
              title="No users"
              description="Create a user to grant access to FreeRouter."
              action={<Button size="sm" onClick={() => setCreateUserOpen(true)}><Plus className="h-4 w-4 mr-1" /> Create User</Button>}
            />
          ) : (
            <Card>
              <CardContent className="p-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="font-mono text-xs">Name</TableHead>
                      <TableHead className="font-mono text-xs">Email</TableHead>
                      <TableHead className="font-mono text-xs">Status</TableHead>
                      <TableHead className="font-mono text-xs w-20" />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {users.map((u) => (
                      <TableRow key={u.id}>
                        <TableCell className="font-medium">{u.name}</TableCell>
                        <TableCell className="text-sm text-muted-foreground">{u.email}</TableCell>
                        <TableCell>
                          <Badge variant={u.active ? "default" : "secondary"}>
                            {u.active ? "Active" : "Suspended"}
                          </Badge>
                        </TableCell>
                        <TableCell className="flex gap-1 justify-end">
                          <Button variant="ghost" size="icon-sm" onClick={() => setEditUser(u)}>
                            <Pencil className="h-4 w-4" />
                          </Button>
                          {u.active && (
                            <Button variant="ghost" size="icon-sm" className="text-destructive" onClick={() => setSuspendUserId(u.id)}>
                              <UserX className="h-4 w-4" />
                            </Button>
                          )}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          )}
        </TabsContent>

        {/* ── Roles Tab ────────────────────────────────────────────── */}
        <TabsContent value="roles" className="space-y-4">
          <div className="flex justify-end">
            <Button size="sm" onClick={() => setCreateRoleOpen(true)}>
              <Plus className="h-4 w-4 mr-1" /> Create Role
            </Button>
          </div>
          {roles.length === 0 ? (
            <EmptyState
              icon={ShieldCheck}
              title="No roles"
              description="Create a role to group permissions for users."
              action={<Button size="sm" onClick={() => setCreateRoleOpen(true)}><Plus className="h-4 w-4 mr-1" /> Create Role</Button>}
            />
          ) : (
            <Card>
              <CardContent className="p-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="font-mono text-xs">Name</TableHead>
                      <TableHead className="font-mono text-xs">Permissions</TableHead>
                      <TableHead className="font-mono text-xs w-20" />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {roles.map((r) => (
                      <TableRow key={r.id}>
                        <TableCell className="font-medium">{r.name}</TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-1">
                            {(r.permissions ?? []).map((p) => (
                              <Badge key={p} variant="outline" className="font-mono text-[10px]">{p}</Badge>
                            ))}
                          </div>
                        </TableCell>
                        <TableCell className="flex gap-1 justify-end">
                          <Button variant="ghost" size="icon-sm" onClick={() => setEditRole(r)}>
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button variant="ghost" size="icon-sm" className="text-destructive" onClick={() => setDeleteRoleId(r.id)}>
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          )}
        </TabsContent>

        {/* ── Assignments Tab ──────────────────────────────────────── */}
        <TabsContent value="assignments" className="space-y-4">
          <div className="flex justify-end">
            <Button size="sm" onClick={() => setAssignOpen(true)}>
              <Plus className="h-4 w-4 mr-1" /> Assign Role
            </Button>
          </div>
          {assignments.length === 0 ? (
            <EmptyState
              icon={Link2}
              title="No role assignments"
              description="Assign roles to users to grant permissions."
              action={<Button size="sm" onClick={() => setAssignOpen(true)}><Plus className="h-4 w-4 mr-1" /> Assign Role</Button>}
            />
          ) : (
            <Card>
              <CardContent className="p-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="font-mono text-xs">User</TableHead>
                      <TableHead className="font-mono text-xs">Role</TableHead>
                      <TableHead className="font-mono text-xs w-8" />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {assignments.map((a) => {
                      const user = users.find((u) => u.id === a.user_id)
                      const role = roles.find((r) => r.id === a.role_id)
                      return (
                        <TableRow key={`${a.user_id}-${a.role_id}`}>
                          <TableCell className="font-medium">
                            {user ? `${user.name} (${user.email})` : a.user_id}
                          </TableCell>
                          <TableCell>
                            <Badge variant="outline">{role?.name ?? a.role_id}</Badge>
                          </TableCell>
                          <TableCell>
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              className="text-destructive"
                              onClick={() => setUnassign({ user_id: a.user_id, role_id: a.role_id })}
                            >
                              <Trash2 className="h-4 w-4" />
                            </Button>
                          </TableCell>
                        </TableRow>
                      )
                    })}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          )}
        </TabsContent>
      </Tabs>

      {/* ── Dialogs ──────────────────────────────────────────────── */}

      <CreateUserDialog
        open={createUserOpen}
        onOpenChange={setCreateUserOpen}
        onCreate={async (req) => {
          await api.access.createUser(req)
          setCreateUserOpen(false)
          toast.success("User created")
          load()
        }}
      />

      <EditUserDialog
        user={editUser}
        onOpenChange={(open) => { if (!open) setEditUser(null) }}
        onSave={async (id, req) => {
          await api.access.updateUser(id, req)
          setEditUser(null)
          toast.success("User updated")
          load()
        }}
      />

      <ConfirmDialog
        open={!!suspendUserId}
        onOpenChange={(open) => { if (!open) setSuspendUserId(null) }}
        title="Suspend User"
        description="This will deactivate the user. They will lose all access until reactivated."
        confirmLabel="Suspend"
        variant="destructive"
        onConfirm={async () => {
          if (!suspendUserId) return
          await api.access.suspendUser(suspendUserId)
          setSuspendUserId(null)
          toast.success("User suspended")
          load()
        }}
      />

      <RoleDialog
        open={createRoleOpen}
        onOpenChange={setCreateRoleOpen}
        onSave={async (req) => {
          await api.access.createRole(req)
          setCreateRoleOpen(false)
          toast.success("Role created")
          load()
        }}
      />

      <RoleDialog
        open={!!editRole}
        onOpenChange={(open) => { if (!open) setEditRole(null) }}
        role={editRole ?? undefined}
        onSave={async (req) => {
          if (!editRole) return
          await api.access.updateRole(editRole.id, req)
          setEditRole(null)
          toast.success("Role updated")
          load()
        }}
      />

      <ConfirmDialog
        open={!!deleteRoleId}
        onOpenChange={(open) => { if (!open) setDeleteRoleId(null) }}
        title="Delete Role"
        description="This will permanently delete the role and remove it from all users."
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={async () => {
          if (!deleteRoleId) return
          await api.access.deleteRole(deleteRoleId)
          setDeleteRoleId(null)
          toast.success("Role deleted")
          load()
        }}
      />

      <AssignDialog
        open={assignOpen}
        onOpenChange={setAssignOpen}
        users={users}
        roles={roles}
        onAssign={async (req) => {
          await api.access.assignRole(req)
          setAssignOpen(false)
          toast.success("Role assigned")
          load()
        }}
      />

      <ConfirmDialog
        open={!!unassign}
        onOpenChange={(open) => { if (!open) setUnassign(null) }}
        title="Unassign Role"
        description="Remove this role from the user?"
        confirmLabel="Unassign"
        variant="destructive"
        onConfirm={async () => {
          if (!unassign) return
          await api.access.unassignRole(unassign)
          setUnassign(null)
          toast.success("Role unassigned")
          load()
        }}
      />
    </div>
  )
}

// =============================================================================
// Create User Dialog
// =============================================================================

function CreateUserDialog({
  open, onOpenChange, onCreate,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreate: (req: CreateUserRequest) => void
}) {
  const [name, setName] = useState("")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")

  useEffect(() => { if (open) { setName(""); setEmail(""); setPassword("") } }, [open])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Create User</DialogTitle>
          <DialogDescription>Add a new user to FreeRouter. Assign them a role after creation.</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="Alice Smith" />
          </div>
          <div className="space-y-2">
            <Label>Email</Label>
            <Input value={email} onChange={(e) => setEmail(e.target.value)} placeholder="alice@example.com" type="email" />
          </div>
          <div className="space-y-2">
            <Label>Password</Label>
            <Input value={password} onChange={(e) => setPassword(e.target.value)} placeholder="Minimum 8 characters" type="password" />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={() => onCreate({ name, email, password })} disabled={!name.trim() || !email.trim() || password.length < 8}>
            Create
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// =============================================================================
// Edit User Dialog
// =============================================================================

function EditUserDialog({
  user, onOpenChange, onSave,
}: {
  user: AccessUser | null
  onOpenChange: (open: boolean) => void
  onSave: (id: string, req: UpdateUserRequest) => void
}) {
  const [name, setName] = useState("")

  useEffect(() => { if (user) setName(user.name) }, [user])

  return (
    <Dialog open={!!user} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Edit User</DialogTitle>
          <DialogDescription>Update the user's profile.</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={() => user && onSave(user.id, { name })} disabled={!name.trim()}>
            Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// =============================================================================
// Role Dialog (create or edit)
// =============================================================================

function RoleDialog({
  open, onOpenChange, onSave, role,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (req: CreateRoleRequest | UpdateRoleRequest) => void
  role?: AccessRole
}) {
  const [name, setName] = useState("")
  const [selectedPerms, setSelectedPerms] = useState<Set<string>>(new Set())

  useEffect(() => {
    if (open) {
      setName(role?.name ?? "")
      setSelectedPerms(new Set(role?.permissions ?? []))
    }
  }, [open, role])

  function toggle(perm: string) {
    setSelectedPerms((prev) => {
      const next = new Set(prev)
      if (next.has(perm)) next.delete(perm)
      else next.add(perm)
      return next
    })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{role ? "Edit Role" : "Create Role"}</DialogTitle>
          <DialogDescription>
            {role ? "Update the role name and permissions." : "Define a role with a set of permissions."}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>Name</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="e.g. Admin, Viewer, Operator" />
          </div>
          <div className="space-y-2">
            <Label>Permissions</Label>
            <div className="grid grid-cols-2 gap-1.5 max-h-48 overflow-y-auto border rounded-md p-2">
              {VALID_PERMISSIONS.map((p) => (
                <label
                  key={p}
                  className="flex items-center gap-2 cursor-pointer text-xs font-mono hover:bg-muted/50 rounded px-1 py-0.5"
                >
                  <input
                    type="checkbox"
                    checked={selectedPerms.has(p)}
                    onChange={() => toggle(p)}
                    className="rounded border-muted-foreground/30"
                  />
                  {p.replace("freerouter:", "")}
                </label>
              ))}
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button
            onClick={() => onSave({ name, permissions: Array.from(selectedPerms) })}
            disabled={!name.trim() || selectedPerms.size === 0}
          >
            {role ? "Save" : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// =============================================================================
// Assign Dialog
// =============================================================================

function AssignDialog({
  open, onOpenChange, users, roles, onAssign,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  users: AccessUser[]
  roles: AccessRole[]
  onAssign: (req: AssignRoleRequest) => void
}) {
  const [userId, setUserId] = useState("")
  const [roleId, setRoleId] = useState("")

  useEffect(() => { if (open) { setUserId(""); setRoleId("") } }, [open])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>Assign Role</DialogTitle>
          <DialogDescription>Grant a role to a user.</DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>User</Label>
            <Select value={userId} onValueChange={(v) => setUserId(v ?? "")}>
              <SelectTrigger><SelectValue placeholder="Select a user" /></SelectTrigger>
              <SelectContent>
                {users.filter((u) => u.active).map((u) => (
                  <SelectItem key={u.id} value={u.id}>{u.name} ({u.email})</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>Role</Label>
            <Select value={roleId} onValueChange={(v) => setRoleId(v ?? "")}>
              <SelectTrigger><SelectValue placeholder="Select a role" /></SelectTrigger>
              <SelectContent>
                {roles.map((r) => (
                  <SelectItem key={r.id} value={r.id}>{r.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>Cancel</Button>
          <Button onClick={() => onAssign({ user_id: userId, role_id: roleId })} disabled={!userId || !roleId}>
            Assign
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
