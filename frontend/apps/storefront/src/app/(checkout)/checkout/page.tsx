"use client";

import React, {
  useState,
  useEffect,
  useCallback,
  useMemo,
  Suspense,
} from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useCartStore } from "@/stores/cart-store";
import { useAuthStore } from "@/stores/auth-store";
import { authFetch, API_BASE_URL, API_KEY } from "@/lib/api";
import { formatCurrency } from "@/lib/format";
import { getImageUrl, extractErrorMessage } from "@/lib/utils";
import {
  ShieldCheck,
  Banknote,
  ArrowLeft,
  ShoppingCart,
  CheckCircle2,
  Loader2,
  MapPin,
  Tag,
  AlertCircle,
  Plus,
  Pencil,
  Trash2,
  X,
  Lock,
  Zap,
} from "lucide-react";
import { toast } from "sonner";
import { useMounted } from "@/hooks/use-mounted";

interface Province {
  province_id: number;
  province_name: string;
  code: string;
}

interface District {
  district_id: number;
  province_id: number;
  district_name: string;
  code: string;
}

interface Ward {
  ward_code: string;
  district_id: number;
  ward_name: string;
}

interface UserAddress {
  uuid: string;
  label: string;
  name: string;
  phone: string;
  address: string;
  province_id?: number;
  province_name?: string;
  district_id?: number;
  district_name?: string;
  ward_code?: string;
  ward_name?: string;
  full_address: string;
  is_default: boolean;
}

function CheckoutContent() {
  const searchParams = useSearchParams();
  const isBuyNowParam = searchParams.get("buyNow") === "1";

  const {
    items,
    buyNowItem,
    clearBuyNowItem,
    removeSelectedItems,
    appliedCoupon,
    setAppliedCoupon,
  } = useCartStore();
  const { user, accessToken } = useAuthStore();

  const isBuyNow = Boolean(isBuyNowParam && buyNowItem);

  const mounted = useMounted();
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [orderPlaced, setOrderPlaced] = useState<{
    orderId: string;
    paymentMethod: string;
    shippingFee: number;
    shippingAddress?: string;
  } | null>(null);

  // Form State
  const [fullName, setFullName] = useState(user?.fullName || "");
  const [phone, setPhone] = useState("");
  const [address, setAddress] = useState("");
  const [note, setNote] = useState("");
  const [paymentMethod, setPaymentMethod] = useState<"COD" | "MOMO">("COD");

  // Cascading Address State (GHN Data for current selection)
  const [provinces, setProvinces] = useState<Province[]>([]);
  const [districts, setDistricts] = useState<District[]>([]);
  const [wards, setWards] = useState<Ward[]>([]);

  const [selectedProvinceId, setSelectedProvinceId] = useState<number | null>(
    null,
  );
  const [selectedDistrictId, setSelectedDistrictId] = useState<number | null>(
    null,
  );
  const [selectedWardCode, setSelectedWardCode] = useState<string>("");

  const [selectedProvinceName, setSelectedProvinceName] = useState<string>("");
  const [selectedDistrictName, setSelectedDistrictName] = useState<string>("");
  const [selectedWardName, setSelectedWardName] = useState<string>("");

  const [isLoadingProvinces, setIsLoadingProvinces] = useState(false);
  const [isLoadingDistricts, setIsLoadingDistricts] = useState(false);
  const [isLoadingWards, setIsLoadingWards] = useState(false);

  // Saved Addresses State
  const [savedAddresses, setSavedAddresses] = useState<UserAddress[]>([]);
  const [selectedSavedAddressUuid, setSelectedSavedAddressUuid] =
    useState<string>("");

  // Modal State for Add / Edit Address
  const [isAddressModalOpen, setIsAddressModalOpen] = useState(false);
  const [editingAddressUuid, setEditingAddressUuid] = useState<string | null>(
    null,
  );
  const [modalName, setModalName] = useState("");
  const [modalPhone, setModalPhone] = useState("");
  const [modalLabel, setModalLabel] = useState("Home");
  const [modalAddress, setModalAddress] = useState("");
  const [modalProvinceId, setModalProvinceId] = useState<number | null>(null);
  const [modalProvinceName, setModalProvinceName] = useState("");
  const [modalDistrictId, setModalDistrictId] = useState<number | null>(null);
  const [modalDistrictName, setModalDistrictName] = useState("");
  const [modalWardCode, setModalWardCode] = useState("");
  const [modalWardName, setModalWardName] = useState("");
  const [modalIsDefault, setModalIsDefault] = useState(false);
  const [modalDistricts, setModalDistricts] = useState<District[]>([]);
  const [modalWards, setModalWards] = useState<Ward[]>([]);
  const [isLoadingModalDistricts, setIsLoadingModalDistricts] = useState(false);
  const [isLoadingModalWards, setIsLoadingModalWards] = useState(false);
  const [isSavingAddress, setIsSavingAddress] = useState(false);
  const [isDeletingAddressUuid, setIsDeletingAddressUuid] = useState<
    string | null
  >(null);

  // Shipping Fee State
  const [shippingFee, setShippingFee] = useState<number | null>(null);
  const [isCalculatingShipping, setIsCalculatingShipping] = useState(false);
  const [shippingError, setShippingError] = useState<string | null>(null);

  const selectedItems = useMemo(
    () =>
      isBuyNow && buyNowItem
        ? [buyNowItem]
        : items.filter((item) => item.selected !== false),
    [isBuyNow, buyNowItem, items],
  );

  const subtotal = useMemo(
    () =>
      selectedItems.reduce((acc, item) => acc + item.price * item.quantity, 0),
    [selectedItems],
  );

  const totalWeight = useMemo(
    () =>
      Math.max(
        200,
        selectedItems.reduce((acc, item) => acc + item.quantity * 250, 0),
      ),
    [selectedItems],
  );

  const discountAmount = appliedCoupon ? appliedCoupon.discountAmount : 0;
  const effectiveShippingFee = shippingFee !== null ? shippingFee : 0;
  const grandTotal = Math.max(
    0,
    subtotal - discountAmount + effectiveShippingFee,
  );

  // Coupon State & Handlers
  const [couponInput, setCouponInput] = useState("");
  const [isApplyingCoupon, setIsApplyingCoupon] = useState(false);

  const handleApplyCoupon = async (
    e?: React.FormEvent | React.MouseEvent | React.KeyboardEvent,
  ) => {
    if (e) e.preventDefault();
    const code = couponInput.trim().toUpperCase();
    if (!code) {
      toast.error("Please enter a valid coupon code.");
      return;
    }

    if (subtotal <= 0) {
      toast.error("Your subtotal must be greater than 0 to apply a coupon.");
      return;
    }

    setIsApplyingCoupon(true);

    try {
      const res = await authFetch('/api/v1/coupons/preview', {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          code,
          subtotal_amount: subtotal,
        }),
      });

      const data = await res.json().catch(() => null);

      if (res.ok && data?.data) {
        setAppliedCoupon({
          code: data.data.coupon_code,
          name: data.data.coupon_name,
          discountAmount: data.data.discount_amount,
        });
        toast.success(
          `Coupon ${data.data.coupon_code} applied! Saved ${formatCurrency(
            data.data.discount_amount,
          )}.`,
        );
        setCouponInput("");
      } else {
        const errMsg =
          data?.detail ||
          data?.error ||
          data?.message ||
          "Invalid or expired coupon code.";
        toast.error(errMsg);
      }
    } catch {
      toast.error("Network error while verifying coupon.");
    } finally {
      setIsApplyingCoupon(false);
    }
  };

  const handleRemoveCoupon = () => {
    setAppliedCoupon(null);
    setCouponInput("");
    toast.info("Coupon removed.");
  };

  // 1. Load Wards when District changes
  const loadWards = useCallback(
    async (
      districtId: number,
      targetWardCode?: string,
      targetWardName?: string,
    ) => {
      setIsLoadingWards(true);
      setWards([]);
      if (!targetWardCode) {
        setSelectedWardCode("");
        setSelectedWardName("");
        setShippingFee(null);
      }

      try {
        const res = await fetch(
          `${API_BASE_URL}/api/v1/shipping/wards?district_id=${districtId}`,
          { headers: { "x-api-key": API_KEY } },
        );
        if (res.ok) {
          const json = await res.json();
          if (json?.data && Array.isArray(json.data)) {
            const sorted = [...json.data].sort((a: Ward, b: Ward) =>
              a.ward_name.localeCompare(b.ward_name),
            );
            setWards(sorted);

            if (targetWardCode) {
              setSelectedWardCode(targetWardCode);
              if (targetWardName) setSelectedWardName(targetWardName);
            }
          }
        }
      } catch {
        toast.error("Failed to load wards from shipping provider.");
      } finally {
        setIsLoadingWards(false);
      }
    },
    [],
  );

  // 2. Load Districts when Province changes
  const loadDistricts = useCallback(
    async (
      provinceId: number,
      targetDistrictId?: number,
      targetWardCode?: string,
      targetWardName?: string,
    ) => {
      setIsLoadingDistricts(true);
      setDistricts([]);
      setWards([]);
      if (!targetDistrictId) {
        setSelectedDistrictId(null);
        setSelectedDistrictName("");
        setSelectedWardCode("");
        setSelectedWardName("");
        setShippingFee(null);
      }

      try {
        const res = await fetch(
          `${API_BASE_URL}/api/v1/shipping/districts?province_id=${provinceId}`,
          { headers: { "x-api-key": API_KEY } },
        );
        if (res.ok) {
          const json = await res.json();
          if (json?.data && Array.isArray(json.data)) {
            const sorted = [...json.data].sort((a: District, b: District) =>
              a.district_name.localeCompare(b.district_name),
            );
            setDistricts(sorted);

            if (targetDistrictId) {
              setSelectedDistrictId(targetDistrictId);
              loadWards(targetDistrictId, targetWardCode, targetWardName);
            }
          }
        }
      } catch {
        toast.error("Failed to load districts from shipping provider.");
      } finally {
        setIsLoadingDistricts(false);
      }
    },
    [loadWards],
  );

  // Helper to populate form from a saved address
  const applySavedAddress = useCallback(
    (addr: UserAddress) => {
      setSelectedSavedAddressUuid(addr.uuid);
      if (addr.name) setFullName(addr.name);
      if (addr.phone) setPhone(addr.phone);
      if (addr.address) setAddress(addr.address);

      if (addr.province_id) {
        setSelectedProvinceId(addr.province_id);
        setSelectedProvinceName(addr.province_name || "");
        loadDistricts(
          addr.province_id,
          addr.district_id,
          addr.ward_code,
          addr.ward_name,
        );
      } else {
        setSelectedProvinceId(null);
        setSelectedProvinceName("");
        setDistricts([]);
        setWards([]);
        setSelectedDistrictId(null);
        setSelectedDistrictName("");
        setSelectedWardCode("");
        setSelectedWardName("");
        setShippingFee(null);
      }
      if (addr.district_name) setSelectedDistrictName(addr.district_name);
    },
    [loadDistricts],
  );

  // 3. Fetch Provinces & Saved Addresses on Mount
  useEffect(() => {
    if (!mounted) return;

    // Fetch Provinces
    setIsLoadingProvinces(true);
    fetch(`${API_BASE_URL}/api/v1/shipping/provinces`, {
      headers: { "x-api-key": API_KEY },
    })
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (data?.data && Array.isArray(data.data)) {
          // Sort alphabetically by province_name
          const sorted = [...data.data].sort((a: Province, b: Province) =>
            a.province_name.localeCompare(b.province_name),
          );
          setProvinces(sorted);
        }
      })
      .catch(() => {})
      .finally(() => setIsLoadingProvinces(false));

    // Fetch Saved Addresses if logged in
    if (accessToken) {
      authFetch('/api/v1/addresses')
        .then((res) => (res.ok ? res.json() : null))
        .then((data) => {
          if (data?.data && Array.isArray(data.data) && data.data.length > 0) {
            setSavedAddresses(data.data);

            // Auto-populate default address
            const defaultAddr =
              data.data.find((a: UserAddress) => a.is_default) || data.data[0];
            if (defaultAddr) {
              applySavedAddress(defaultAddr);
            }
          } else {
            setSavedAddresses([]);
          }
        })
        .catch(() => {});
    }
  }, [mounted, accessToken, applySavedAddress]);

  // Load Wards for Modal
  const loadModalWards = useCallback(
    async (
      districtId: number,
      targetWardCode?: string,
      targetWardName?: string,
    ) => {
      setIsLoadingModalWards(true);
      setModalWards([]);
      try {
        const res = await fetch(
          `${API_BASE_URL}/api/v1/shipping/wards?district_id=${districtId}`,
          { headers: { "x-api-key": API_KEY } },
        );
        if (res.ok) {
          const json = await res.json();
          if (json?.data && Array.isArray(json.data)) {
            const sorted = [...json.data].sort((a: Ward, b: Ward) =>
              a.ward_name.localeCompare(b.ward_name),
            );
            setModalWards(sorted);

            if (targetWardCode) {
              setModalWardCode(targetWardCode);
              if (targetWardName) setModalWardName(targetWardName);
            }
          }
        }
      } catch {
        toast.error("Failed to load wards.");
      } finally {
        setIsLoadingModalWards(false);
      }
    },
    [],
  );

  // Load Districts for Modal
  const loadModalDistricts = useCallback(
    async (
      provinceId: number,
      targetDistrictId?: number,
      targetWardCode?: string,
      targetWardName?: string,
    ) => {
      setIsLoadingModalDistricts(true);
      setModalDistricts([]);
      setModalWards([]);
      try {
        const res = await fetch(
          `${API_BASE_URL}/api/v1/shipping/districts?province_id=${provinceId}`,
          { headers: { "x-api-key": API_KEY } },
        );
        if (res.ok) {
          const json = await res.json();
          if (json?.data && Array.isArray(json.data)) {
            const sorted = [...json.data].sort((a: District, b: District) =>
              a.district_name.localeCompare(b.district_name),
            );
            setModalDistricts(sorted);

            if (targetDistrictId) {
              setModalDistrictId(targetDistrictId);
              loadModalWards(targetDistrictId, targetWardCode, targetWardName);
            }
          }
        }
      } catch {
        toast.error("Failed to load districts.");
      } finally {
        setIsLoadingModalDistricts(false);
      }
    },
    [loadModalWards],
  );

  const openCreateAddressModal = () => {
    setEditingAddressUuid(null);
    setModalName(fullName || user?.fullName || "");
    setModalPhone(phone || "");
    setModalLabel("Home");
    setModalAddress("");
    setModalProvinceId(null);
    setModalProvinceName("");
    setModalDistrictId(null);
    setModalDistrictName("");
    setModalWardCode("");
    setModalWardName("");
    setModalDistricts([]);
    setModalWards([]);
    setModalIsDefault(savedAddresses.length === 0);
    setIsAddressModalOpen(true);
  };

  const openEditAddressModal = (addr: UserAddress, e?: React.MouseEvent) => {
    if (e) e.stopPropagation();
    setEditingAddressUuid(addr.uuid);
    setModalName(addr.name || "");
    setModalPhone(addr.phone || "");
    setModalLabel(addr.label || "Home");
    setModalAddress(addr.address || "");
    setModalProvinceId(addr.province_id || null);
    setModalProvinceName(addr.province_name || "");
    setModalDistrictId(addr.district_id || null);
    setModalDistrictName(addr.district_name || "");
    setModalWardCode(addr.ward_code || "");
    setModalWardName(addr.ward_name || "");
    setModalIsDefault(Boolean(addr.is_default));
    setModalDistricts([]);
    setModalWards([]);

    if (addr.province_id) {
      loadModalDistricts(
        addr.province_id,
        addr.district_id,
        addr.ward_code,
        addr.ward_name,
      );
    }
    setIsAddressModalOpen(true);
  };

  const handleModalProvinceChange = (
    e: React.ChangeEvent<HTMLSelectElement>,
  ) => {
    const provId = Number(e.target.value);
    if (!provId) {
      setModalProvinceId(null);
      setModalProvinceName("");
      setModalDistricts([]);
      setModalWards([]);
      setModalDistrictId(null);
      setModalDistrictName("");
      setModalWardCode("");
      setModalWardName("");
      return;
    }
    const found = provinces.find((p) => p.province_id === provId);
    setModalProvinceId(provId);
    setModalProvinceName(found?.province_name || "");
    setModalDistrictId(null);
    setModalDistrictName("");
    setModalWardCode("");
    setModalWardName("");
    setModalWards([]);
    loadModalDistricts(provId);
  };

  const handleModalDistrictChange = (
    e: React.ChangeEvent<HTMLSelectElement>,
  ) => {
    const distId = Number(e.target.value);
    if (!distId) {
      setModalDistrictId(null);
      setModalDistrictName("");
      setModalWards([]);
      setModalWardCode("");
      setModalWardName("");
      return;
    }
    const found = modalDistricts.find((d) => d.district_id === distId);
    setModalDistrictId(distId);
    setModalDistrictName(found?.district_name || "");
    setModalWardCode("");
    setModalWardName("");
    loadModalWards(distId);
  };

  const handleModalWardChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const code = e.target.value;
    if (!code) {
      setModalWardCode("");
      setModalWardName("");
      return;
    }
    const found = modalWards.find((w) => w.ward_code === code);
    setModalWardCode(code);
    setModalWardName(found?.ward_name || "");
  };

  const handleSaveAddress = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!accessToken) {
      toast.error("Please sign in to save address.");
      return;
    }

    const trimmedName = modalName.trim();
    if (trimmedName.length < 2) {
      toast.error("Full Name must be at least 2 characters.");
      return;
    }

    const cleanPhone = modalPhone.trim().replace(/[\s.-]+/g, "");
    if (cleanPhone.length < 9) {
      toast.error("Phone number must be at least 9 digits.");
      return;
    }

    const trimmedAddress = modalAddress.trim();
    if (trimmedAddress.length < 5) {
      toast.error(
        "Street address must be at least 5 characters (e.g. house number, street name).",
      );
      return;
    }

    const finalProvName =
      modalProvinceName ||
      provinces.find((p) => p.province_id === modalProvinceId)?.province_name ||
      "";
    if (!modalProvinceId || !finalProvName) {
      toast.error("Please select a City / Province.");
      return;
    }

    const finalDistName =
      modalDistrictName ||
      modalDistricts.find((d) => d.district_id === modalDistrictId)
        ?.district_name ||
      "";
    if (!modalDistrictId || !finalDistName) {
      toast.error("Please select a District.");
      return;
    }

    const finalWardName =
      modalWardName ||
      modalWards.find((w) => String(w.ward_code) === String(modalWardCode))
        ?.ward_name ||
      "";
    if (!modalWardCode || !finalWardName) {
      toast.error("Please select a Ward.");
      return;
    }

    setIsSavingAddress(true);
    const baseUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

    const payload = {
      label: modalLabel || "Home",
      name: trimmedName,
      phone: cleanPhone,
      address: trimmedAddress,
      province_id: Number(modalProvinceId),
      province_name: finalProvName,
      district_id: Number(modalDistrictId),
      district_name: finalDistName,
      ward_code: String(modalWardCode),
      ward_name: finalWardName,
      is_default: modalIsDefault,
    };

    try {
      const url = editingAddressUuid
        ? `${baseUrl}/api/v1/addresses/${editingAddressUuid}`
        : `${baseUrl}/api/v1/addresses`;
      const method = editingAddressUuid ? "PUT" : "POST";

      const res = await authFetch(url, {
        method,
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(payload),
      });

      if (!res.ok) {
        if (res.status === 401) {
          throw new Error("Your session has expired. Please sign in again.");
        }
        const errJson = await res.json().catch(() => null);
        throw new Error(
          extractErrorMessage(errJson, "Failed to save address."),
        );
      }

      const json = await res.json();
      const savedAddr: UserAddress = json.data;

      // If updating and marked default, trigger default patch
      if (editingAddressUuid && modalIsDefault) {
        await authFetch(
          `${baseUrl}/api/v1/addresses/${editingAddressUuid}/default`,
          {
            method: "PATCH",
          },
        ).catch(() => {});
      }

      // Refresh address list from server
      const listRes = await authFetch(`${baseUrl}/api/v1/addresses`);

      if (listRes.ok) {
        const listJson = await listRes.json();
        if (listJson?.data && Array.isArray(listJson.data)) {
          setSavedAddresses(listJson.data);
          const found =
            listJson.data.find(
              (a: UserAddress) =>
                a.uuid === (savedAddr?.uuid || editingAddressUuid),
            ) || savedAddr;
          applySavedAddress(found);
        }
      } else {
        setSavedAddresses((prev) => {
          if (editingAddressUuid) {
            return prev.map((a) => (a.uuid === savedAddr.uuid ? savedAddr : a));
          }
          return [savedAddr, ...prev];
        });
        applySavedAddress(savedAddr);
      }

      setIsAddressModalOpen(false);
      toast.success(
        editingAddressUuid
          ? "Address updated successfully!"
          : "Address added successfully!",
      );
    } catch (err: unknown) {
      const msg =
        err instanceof Error ? err.message : "Failed to save address.";
      toast.error(msg);
    } finally {
      setIsSavingAddress(false);
    }
  };

  const handleDeleteAddress = async (uuid: string, e: React.MouseEvent) => {
    e.stopPropagation();
    if (!accessToken) return;

    if (!window.confirm("Are you sure you want to delete this address?")) {
      return;
    }

    setIsDeletingAddressUuid(uuid);
    const baseUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

    try {
      const res = await authFetch(`${baseUrl}/api/v1/addresses/${uuid}`, {
        method: "DELETE",
      });

      if (!res.ok) {
        if (res.status === 401) {
          throw new Error("Your session has expired. Please sign in again.");
        }
        const errJson = await res.json().catch(() => null);
        throw new Error(
          extractErrorMessage(errJson, "Failed to delete address."),
        );
      }

      setSavedAddresses((prev) => {
        const next = prev.filter((a) => a.uuid !== uuid);
        if (selectedSavedAddressUuid === uuid) {
          if (next.length > 0) {
            applySavedAddress(next[0]);
          } else {
            setSelectedSavedAddressUuid("");
            setFullName("");
            setPhone("");
            setAddress("");
            setSelectedProvinceId(null);
            setSelectedProvinceName("");
            setSelectedDistrictId(null);
            setSelectedDistrictName("");
            setSelectedWardCode("");
            setSelectedWardName("");
            setDistricts([]);
            setWards([]);
            setShippingFee(null);
          }
        }
        return next;
      });

      toast.success("Address deleted successfully.");
    } catch (err: unknown) {
      const msg =
        err instanceof Error ? err.message : "Failed to delete address.";
      toast.error(msg);
    } finally {
      setIsDeletingAddressUuid(null);
    }
  };

  // 4. Calculate Real Shipping Fee when district & ward are selected
  useEffect(() => {
    if (
      !selectedDistrictId ||
      !selectedWardCode ||
      selectedItems.length === 0
    ) {
      setShippingFee(null);
      setShippingError(null);
      return;
    }

    let isMounted = true;
    const calculateFee = async () => {
      setIsCalculatingShipping(true);
      setShippingError(null);

      try {
        const res = await fetch(`${API_BASE_URL}/api/v1/shipping/calculate-fee`, {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "x-api-key": API_KEY,
          },
          body: JSON.stringify({
            provider: "ghn",
            district_id: selectedDistrictId,
            ward_code: selectedWardCode,
            weight: totalWeight,
            insurance_value: subtotal,
          }),
        });

        if (!isMounted) return;

        if (res.ok) {
          const json = await res.json();
          if (json?.data?.shipping_fee !== undefined) {
            setShippingFee(json.data.shipping_fee);
          }
        } else {
          // Fallback if carrier fails
          setShippingFee(30000);
          setShippingError(
            "GHN rate unavailable. Applied standard delivery rate.",
          );
        }
      } catch {
        if (isMounted) {
          setShippingFee(30000);
          setShippingError("Network error. Applied standard delivery rate.");
        }
      } finally {
        if (isMounted) {
          setIsCalculatingShipping(false);
        }
      }
    };

    const timer = setTimeout(calculateFee, 300);
    return () => {
      isMounted = false;
      clearTimeout(timer);
    };
  }, [
    selectedDistrictId,
    selectedWardCode,
    totalWeight,
    subtotal,
    selectedItems.length,
  ]);

  // 5. Submit Order
  const handleSubmitOrder = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!accessToken) {
      toast.error("Please sign in to place your order.");
      return;
    }

    if (selectedItems.length === 0) {
      toast.error("No items selected for checkout.");
      return;
    }

    if (savedAddresses.length === 0) {
      toast.error("Please add a shipping address to place your order.");
      return;
    }

    if (!selectedSavedAddressUuid) {
      toast.error("Please select a shipping address.");
      return;
    }

    if (
      !fullName.trim() ||
      !phone.trim() ||
      !address.trim() ||
      !selectedProvinceName ||
      !selectedDistrictName ||
      !selectedWardName
    ) {
      toast.error(
        "Please select a complete shipping address (Province, District, Ward).",
      );
      return;
    }

    setIsSubmitting(true);

    try {
      const fullShippingAddress = `${address.trim()}, ${selectedWardName}, ${selectedDistrictName}, ${selectedProvinceName}`;

      const orderPayload = {
        shipping_name: fullName.trim(),
        shipping_phone: phone.trim(),
        shipping_address: fullShippingAddress,
        shipping_fee: effectiveShippingFee,
        payment_method: paymentMethod === "COD" ? 1 : 3,
        note: note.trim() || undefined,
        coupon_code: appliedCoupon ? appliedCoupon.code : undefined,
        items: selectedItems.map((item) => ({
          product_uuid: item.productUuid,
          quantity: item.quantity,
        })),
      };

      const res = await authFetch('/api/v1/orders', {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(orderPayload),
      }).catch(() => null);

      let generatedId = `GS-${Math.floor(100000 + Math.random() * 900000)}`;
      let createdOrderUuid = "";
      let finalShippingFee = effectiveShippingFee;

      if (res && res.ok) {
        const data = await res.json();
        if (data?.data?.uuid) {
          createdOrderUuid = data.data.uuid;
          generatedId =
            data.data.order_number || data.data.uuid.slice(0, 8).toUpperCase();
          if (typeof data.data.shipping_fee === "number") {
            finalShippingFee = data.data.shipping_fee;
          }
        }
      } else if (res && !res.ok) {
        const errJson = await res.json().catch(() => null);
        const errMsg = extractErrorMessage(errJson, "Failed to place order.");
        toast.error(errMsg);
        setIsSubmitting(false);
        return;
      }

      // If MoMo payment method is selected and order was created, initiate MoMo payment intent
      if (paymentMethod === "MOMO" && createdOrderUuid) {
        try {
          const paymentRes = await authFetch(
            '/api/v1/payments/create-intent',
            {
              method: "POST",
              headers: {
                "Content-Type": "application/json",
                "X-Idempotency-Key": `idem-pay-${createdOrderUuid}`,
              },
              body: JSON.stringify({
                order_uuid: createdOrderUuid,
                provider: "momo",
              }),
            },
          ).catch(() => null);

          if (paymentRes && paymentRes.ok) {
            const paymentData = await paymentRes.json();
            const paymentUrl = paymentData?.data?.payment_url;
            if (paymentUrl) {
              if (isBuyNow) {
                clearBuyNowItem();
              } else {
                removeSelectedItems();
              }
              setAppliedCoupon(null);
              toast.success("Redirecting to MoMo payment gateway...");
              window.location.href = paymentUrl;
              return;
            }
          } else {
            const payErr = paymentRes
              ? await paymentRes.json().catch(() => null)
              : null;
            const payMsg = extractErrorMessage(
              payErr,
              "Could not initiate MoMo payment URL.",
            );
            toast.error(payMsg);
          }
        } catch {
          toast.error("Network error while connecting to MoMo gateway.");
        }
      }


      if (isBuyNow) {
        clearBuyNowItem();
      } else {
        removeSelectedItems();
      }
      setAppliedCoupon(null);
      setOrderPlaced({
        orderId: generatedId,
        paymentMethod,
        shippingFee: finalShippingFee,
        shippingAddress: fullShippingAddress,
      });
      toast.success("Order placed successfully!");
    } catch {
      toast.error("Failed to create order. Please try again.");
    } finally {
      setIsSubmitting(false);
    }
  };

  if (!mounted) {
    return (
      <div className="min-h-[60vh] flex items-center justify-center">
        <div className="w-8 h-8 rounded-full border-2 border-[#2196F3] border-t-transparent animate-spin" />
      </div>
    );
  }

  if (!user || !accessToken) {
    return (
      <div className="min-h-[70vh] bg-[#FAFBFC] py-16 flex items-center justify-center">
        <div className="max-w-md w-full mx-auto px-4 text-center space-y-4">
          <div className="w-20 h-20 rounded-3xl bg-[#E3F2FD] text-[#2196F3] flex items-center justify-center mx-auto shadow-xs">
            <Lock className="w-10 h-10" />
          </div>
          <h1 className="text-2xl font-extrabold text-[#0D1B2A] tracking-tight">
            Sign In Required
          </h1>
          <p className="text-sm text-slate-500 leading-relaxed">
            Please sign in to your GOshop account to proceed with checkout.
          </p>
          <div className="pt-2 flex flex-col sm:flex-row items-center justify-center gap-3">
            <Link
              href="/login?redirect=/checkout"
              className="w-full sm:w-auto px-6 py-3 rounded-2xl bg-[#2196F3] text-white text-sm font-semibold hover:bg-[#0D47A1] transition-all shadow-md text-center"
            >
              Sign In Now
            </Link>
            <Link
              href="/cart"
              className="w-full sm:w-auto px-6 py-3 rounded-2xl border border-slate-200 bg-white text-slate-700 text-sm font-semibold hover:bg-slate-50 transition-all text-center"
            >
              Back to Cart
            </Link>
          </div>
        </div>
      </div>
    );
  }

  if (!orderPlaced && selectedItems.length === 0) {
    return (
      <div className="min-h-[70vh] bg-[#FAFBFC] py-16 flex items-center justify-center">
        <div className="max-w-md w-full mx-auto px-4 text-center space-y-4">
          <div className="w-20 h-20 rounded-3xl bg-[#E3F2FD] text-[#2196F3] flex items-center justify-center mx-auto shadow-xs">
            <ShoppingCart className="w-10 h-10" />
          </div>
          <h1 className="text-2xl font-extrabold text-[#0D1B2A] tracking-tight">
            No Items Selected for Checkout
          </h1>
          <p className="text-sm text-slate-500 leading-relaxed">
            Please choose the products you wish to purchase from your shopping
            cart.
          </p>
          <div className="pt-2">
            <Link
              href="/cart"
              className="inline-flex items-center gap-2 px-6 py-3 rounded-2xl bg-[#2196F3] text-white text-sm font-semibold hover:bg-[#0D47A1] transition-all shadow-md"
            >
              <ArrowLeft className="w-4 h-4" />
              <span>Back to Cart</span>
            </Link>
          </div>
        </div>
      </div>
    );
  }

  // Order Confirmation Screen
  if (orderPlaced) {
    return (
      <div className="bg-[#FAFBFC] min-h-[75vh] py-16 flex items-center justify-center">
        <div className="max-w-lg w-full mx-auto px-4 text-center space-y-5">
          <div className="w-20 h-20 rounded-full bg-emerald-50 text-emerald-500 flex items-center justify-center mx-auto shadow-xs">
            <CheckCircle2 className="w-12 h-12" />
          </div>
          <h1 className="text-3xl font-black text-[#0D1B2A] tracking-tight">
            Thank You For Your Order!
          </h1>
          <p className="text-sm text-slate-600 leading-relaxed">
            Your order{" "}
            <strong className="text-[#0D47A1]">#{orderPlaced.orderId}</strong>{" "}
            has been placed successfully.
          </p>

          <div className="p-4 rounded-2xl bg-white border border-slate-200/80 text-left space-y-2 text-xs">
            <div className="flex justify-between">
              <span className="text-slate-500">Payment Mode:</span>
              <span className="font-bold text-[#0D1B2A]">
                {orderPlaced.paymentMethod === "COD"
                  ? "Cash on Delivery (COD)"
                  : "MoMo E-Wallet"}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-500">Delivery Address:</span>
              <span
                className="font-medium text-slate-700 truncate max-w-xs"
                title={orderPlaced.shippingAddress}
              >
                {orderPlaced.shippingAddress || `${address}, ${selectedWardName}, ${selectedDistrictName}, ${selectedProvinceName}`}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-500">Shipping Partner:</span>
              <span className="font-semibold text-emerald-600">
                GHN Express ({formatCurrency(orderPlaced.shippingFee)})
              </span>
            </div>
          </div>

          <div className="pt-4 flex flex-col sm:flex-row gap-3 justify-center">
            <Link
              href="/"
              className="px-6 py-3 rounded-xl bg-[#2196F3] text-white text-xs font-bold hover:bg-[#0D47A1] transition-all shadow-md"
            >
              Back to Home
            </Link>
            <Link
              href="/products"
              className="px-6 py-3 rounded-xl bg-white border border-slate-200 text-slate-700 text-xs font-semibold hover:bg-slate-50 transition-all"
            >
              Continue Shopping
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-[#FAFBFC] min-h-screen py-3">
      <div className="max-w-6xl mx-auto px-4 sm:px-6">
        {/* Header */}
        <div className="flex items-center justify-between pb-6 border-b border-slate-200/80 mb-8">
          <div>
            <Link
              href={
                isBuyNow && buyNowItem
                  ? `/products/${buyNowItem.productUuid}`
                  : "/cart"
              }
              className="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-500 hover:text-[#2196F3] mb-2"
            >
              <ArrowLeft className="w-3.5 h-3.5" />
              <span>{isBuyNow ? "Back to product" : "Back to cart"}</span>
            </Link>
            <div className="flex items-center gap-3">
              <h1 className="text-2xl sm:text-3xl font-extrabold text-[#0D1B2A] tracking-tight">
                Checkout
              </h1>
              {isBuyNow && (
                <span className="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-[#E3F2FD] text-[#0D47A1] text-xs font-bold border border-[#90CAF9]/50 shadow-2xs">
                  <Zap className="w-3.5 h-3.5 text-[#2196F3]" />
                  Direct Buy Now
                </span>
              )}
            </div>
          </div>
        </div>

        {/* Not Logged In Warning Banner */}
        {!accessToken && (
          <div className="mb-6 p-4 rounded-2xl bg-amber-50 border border-amber-200 flex items-center justify-between gap-4">
            <div className="flex items-center gap-2.5 text-xs text-amber-800 font-medium">
              <AlertCircle className="w-4 h-4 text-amber-600 shrink-0" />
              <span>
                Please sign in to access your saved addresses, apply discount
                coupons, and complete order placement.
              </span>
            </div>
            <Link
              href="/login?redirect=/checkout"
              className="shrink-0 px-3.5 py-1.5 rounded-xl bg-amber-600 text-white text-xs font-bold hover:bg-amber-700 transition-colors shadow-xs"
            >
              Sign In
            </Link>
          </div>
        )}

        <form onSubmit={handleSubmitOrder}>
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
            {/* Left Column: Form Details (7 cols) */}
            <div className="lg:col-span-7 space-y-6">
              {/* Select Shipping Address Section */}
              <div className="p-6 rounded-2xl bg-white border border-slate-200/80 shadow-xs space-y-5">
                {/* Header */}
                <div className="flex items-center justify-between pb-3 border-b border-slate-100">
                  <div className="flex items-center gap-2.5">
                    <div className="w-8 h-8 rounded-xl bg-[#E3F2FD] text-[#2196F3] flex items-center justify-center">
                      <MapPin className="w-4 h-4" />
                    </div>
                    <div>
                      <h2 className="text-base font-bold text-[#0D1B2A]">
                        Select Shipping Address
                      </h2>
                      <p className="text-xs text-slate-500">
                        Choose a delivery destination or add a new address
                      </p>
                    </div>
                  </div>
                  {savedAddresses.length > 0 && (
                    <button
                      type="button"
                      onClick={openCreateAddressModal}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-[#2196F3] text-white text-xs font-semibold hover:bg-[#0D47A1] transition-all shadow-xs cursor-pointer"
                    >
                      <Plus className="w-3.5 h-3.5" />
                      <span>Add New Address</span>
                    </button>
                  )}
                </div>

                {/* Case 1: User has saved addresses -> Display list */}
                {savedAddresses.length > 0 ? (
                  <div className="space-y-3">
                    {savedAddresses.map((addr) => {
                      const isSelected = selectedSavedAddressUuid === addr.uuid;
                      const isDeleting = isDeletingAddressUuid === addr.uuid;
                      const hasGhnData = Boolean(
                        addr.province_id && addr.district_id && addr.ward_code,
                      );

                      return (
                        <div
                          key={addr.uuid}
                          onClick={() => applySavedAddress(addr)}
                          className={`p-4 rounded-xl border-2 transition-all cursor-pointer relative flex flex-col sm:flex-row sm:items-center justify-between gap-3 ${
                            isSelected
                              ? "border-[#2196F3] bg-[#E3F2FD]/30 shadow-xs"
                              : "border-slate-200 bg-white hover:border-slate-300 hover:bg-slate-50/60"
                          }`}
                        >
                          <div className="flex items-start gap-3 flex-1 min-w-0">
                            {/* Radio / Tick Icon */}
                            <div
                              className={`w-5 h-5 rounded-full border-2 mt-0.5 shrink-0 flex items-center justify-center transition-colors ${
                                isSelected
                                  ? "border-[#2196F3] bg-[#2196F3]"
                                  : "border-slate-300 bg-white"
                              }`}
                            >
                              {isSelected && (
                                <div className="w-2 h-2 rounded-full bg-white" />
                              )}
                            </div>

                            <div className="space-y-1 min-w-0 flex-1">
                              <div className="flex items-center gap-2 flex-wrap">
                                <span className="text-xs font-bold text-[#0D1B2A]">
                                  {addr.name}
                                </span>
                                <span className="text-xs text-slate-500 font-medium">
                                  {addr.phone}
                                </span>
                                {addr.label && (
                                  <span className="text-[10px] font-medium px-2 py-0.5 rounded-md bg-slate-100 text-slate-600 border border-slate-200">
                                    {addr.label}
                                  </span>
                                )}
                                {addr.is_default && (
                                  <span className="text-[10px] font-bold px-2 py-0.5 rounded-md bg-[#2196F3] text-white">
                                    Default
                                  </span>
                                )}
                                {!hasGhnData && (
                                  <span className="text-[10px] font-medium px-2 py-0.5 rounded-md bg-amber-50 text-amber-700 border border-amber-200">
                                    Incomplete Area (Click Edit)
                                  </span>
                                )}
                              </div>

                              <p className="text-xs text-slate-600 leading-relaxed">
                                {addr.full_address || addr.address}
                              </p>
                            </div>
                          </div>

                          {/* Action Buttons: Edit & Delete */}
                          <div className="flex items-center gap-2 shrink-0 self-end sm:self-center pt-2 sm:pt-0 border-t sm:border-t-0 border-slate-100 w-full sm:w-auto justify-end">
                            <button
                              type="button"
                              onClick={(e) => openEditAddressModal(addr, e)}
                              className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg border border-slate-200 text-slate-700 hover:bg-slate-100 text-xs font-medium transition-colors cursor-pointer"
                              title="Edit address"
                            >
                              <Pencil className="w-3.5 h-3.5 text-slate-500" />
                              <span>Edit</span>
                            </button>
                            <button
                              type="button"
                              disabled={isDeleting}
                              onClick={(e) => handleDeleteAddress(addr.uuid, e)}
                              className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg border border-rose-200 text-rose-600 hover:bg-rose-50 text-xs font-medium transition-colors disabled:opacity-50 cursor-pointer"
                              title="Delete address"
                            >
                              {isDeleting ? (
                                <Loader2 className="w-3.5 h-3.5 animate-spin" />
                              ) : (
                                <Trash2 className="w-3.5 h-3.5" />
                              )}
                              <span>Delete</span>
                            </button>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                ) : (
                  /* Case 2: User has NO saved addresses -> Show notification & add button */
                  <div className="p-8 text-center rounded-2xl border-2 border-dashed border-slate-200 bg-slate-50/50 flex flex-col items-center justify-center gap-3">
                    <div className="w-12 h-12 rounded-full bg-slate-100 text-slate-400 flex items-center justify-center">
                      <MapPin className="w-6 h-6" />
                    </div>
                    <div className="space-y-1">
                      <h3 className="text-sm font-bold text-[#0D1B2A]">
                        No Shipping Address Found
                      </h3>
                      <p className="text-xs text-slate-500 max-w-sm">
                        You don&apos;t have any saved delivery address yet.
                        Please add an address to proceed with checkout.
                      </p>
                    </div>
                    <button
                      type="button"
                      onClick={openCreateAddressModal}
                      className="mt-2 inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-[#2196F3] text-white text-xs font-bold hover:bg-[#0D47A1] transition-all shadow-sm cursor-pointer"
                    >
                      <Plus className="w-4 h-4" />
                      <span>Add Shipping Address</span>
                    </button>
                  </div>
                )}

                {/* Delivery Note */}
                <div className="space-y-1.5 pt-3 border-t border-slate-100">
                  <label className="text-xs font-semibold text-slate-700">
                    Delivery Instructions / Note
                  </label>
                  <textarea
                    rows={2}
                    value={note}
                    onChange={(e) => setNote(e.target.value)}
                    placeholder="e.g. Call before delivery, drop at reception desk"
                    className="w-full px-3.5 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                  />
                </div>
              </div>

              {/* Payment Methods */}
              <div className="p-6 rounded-2xl bg-white border border-slate-200/80 shadow-xs space-y-4">
                <h2 className="text-base font-bold text-[#0D1B2A]">
                  Payment Method
                </h2>

                <div className="space-y-3">
                  {/* COD */}
                  <label
                    onClick={() => setPaymentMethod("COD")}
                    className={`flex items-start gap-3 p-4 rounded-xl border cursor-pointer transition-all ${
                      paymentMethod === "COD"
                        ? "border-[#2196F3] bg-[#E3F2FD]/40 shadow-xs"
                        : "border-slate-200 hover:bg-slate-50"
                    }`}
                  >
                    <input
                      type="radio"
                      name="payment"
                      checked={paymentMethod === "COD"}
                      onChange={() => setPaymentMethod("COD")}
                      className="mt-0.5 text-[#2196F3]"
                    />
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <Banknote className="w-4 h-4 text-[#0D47A1]" />
                        <span className="text-xs font-bold text-[#0D1B2A]">
                          Cash on Delivery (COD)
                        </span>
                      </div>
                      <p className="text-[11px] text-slate-500 mt-1">
                        Pay in cash upon receiving your package from the
                        courier.
                      </p>
                    </div>
                  </label>

                  {/* MoMo E-Wallet */}
                  <label
                    onClick={() => setPaymentMethod("MOMO")}
                    className={`flex items-start gap-3 p-4 rounded-xl border cursor-pointer transition-all ${
                      paymentMethod === "MOMO"
                        ? "border-[#D82D8B] bg-[#FFF0F6] shadow-xs"
                        : "border-slate-200 hover:bg-slate-50"
                    }`}
                  >
                    <input
                      type="radio"
                      name="payment"
                      checked={paymentMethod === "MOMO"}
                      onChange={() => setPaymentMethod("MOMO")}
                      className="mt-0.5 text-[#D82D8B] accent-[#D82D8B]"
                    />
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <div className="w-5 h-5 rounded-md bg-[#A50064] text-white font-black text-[11px] flex items-center justify-center tracking-tighter shrink-0">
                          M
                        </div>
                        <span className="text-xs font-bold text-[#0D1B2A]">
                          MoMo E-Wallet
                        </span>
                        <span className="px-2 py-0.5 rounded-full bg-pink-100 text-[#A50064] text-[10px] font-bold">
                          QR & App
                        </span>
                      </div>
                      <p className="text-[11px] text-slate-500 mt-1">
                        Scan QR code with MoMo App or pay directly with MoMo
                        E-Wallet.
                      </p>
                    </div>
                  </label>
                </div>
              </div>
            </div>

            {/* Right Column: Order Items & Summary (5 cols) */}
            <div className="lg:col-span-5 space-y-4">
              <div className="p-6 rounded-2xl bg-white border border-slate-200/80 shadow-xs space-y-5 sticky top-24">
                <h2 className="text-base font-bold text-[#0D1B2A]">
                  Order Items ({selectedItems.length})
                </h2>

                {/* Items List */}
                <div className="max-h-60 overflow-y-auto divide-y divide-slate-100 pr-1">
                  {selectedItems.map((item) => (
                    <div key={item.id} className="py-3 flex items-center gap-3">
                      <div className="w-12 h-12 rounded-lg bg-slate-100 overflow-hidden shrink-0 border border-slate-200/60 flex items-center justify-center">
                        {item.image ? (
                          <img
                            src={getImageUrl(item.image)}
                            alt={item.name}
                            className="w-full h-full object-cover"
                          />
                        ) : (
                          <ShoppingCart className="w-4 h-4 text-slate-400" />
                        )}
                      </div>

                      <div className="flex-1 min-w-0">
                        <h4 className="text-xs font-semibold text-[#0D1B2A] truncate">
                          {item.name}
                        </h4>
                        {(item.selectedSize || item.selectedColor) && (
                          <p className="text-[10px] text-slate-400 mt-0.5">
                            {item.selectedSize && `Size: ${item.selectedSize}`}
                            {item.selectedSize && item.selectedColor && " | "}
                            {item.selectedColor &&
                              `Color: ${item.selectedColor}`}
                          </p>
                        )}
                        <p className="text-[11px] text-slate-500">
                          Qty: {item.quantity} × {formatCurrency(item.price)}
                        </p>
                      </div>

                      <span className="text-xs font-bold text-[#0D47A1]">
                        {formatCurrency(item.price * item.quantity)}
                      </span>
                    </div>
                  ))}
                </div>

                {/* Promo / Coupon Code Section */}
                <div className="pt-3 border-t border-slate-100 space-y-2.5">
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-bold text-[#0D1B2A] flex items-center gap-1.5">
                      <Tag className="w-3.5 h-3.5 text-[#2196F3]" />
                      Promo / Voucher Code
                    </span>
                    {appliedCoupon && (
                      <span className="text-[11px] font-semibold text-emerald-600 bg-emerald-50 px-2 py-0.5 rounded-full border border-emerald-200">
                        1 Applied
                      </span>
                    )}
                  </div>

                  {appliedCoupon ? (
                    <div className="p-3 rounded-xl bg-emerald-50/80 border border-emerald-200 text-xs text-emerald-900 flex items-center justify-between transition-all">
                      <div className="flex items-center gap-2.5 min-w-0">
                        <div className="w-7 h-7 rounded-lg bg-emerald-500 text-white flex items-center justify-center shrink-0 shadow-2xs">
                          <Tag className="w-3.5 h-3.5" />
                        </div>
                        <div className="min-w-0">
                          <div className="flex items-center gap-2">
                            <span className="font-extrabold text-emerald-800 tracking-wide font-mono">
                              {appliedCoupon.code}
                            </span>
                            <span className="text-emerald-700 font-bold">
                              -{formatCurrency(discountAmount)}
                            </span>
                          </div>
                          {appliedCoupon.name && (
                            <p className="text-[11px] text-emerald-700 truncate mt-0.5">
                              {appliedCoupon.name}
                            </p>
                          )}
                        </div>
                      </div>
                      <button
                        type="button"
                        onClick={handleRemoveCoupon}
                        className="p-1.5 rounded-lg text-slate-400 hover:text-rose-600 hover:bg-rose-50 transition-colors cursor-pointer shrink-0 ml-2"
                        title="Remove coupon"
                      >
                        <X className="w-4 h-4" />
                      </button>
                    </div>
                  ) : (
                    <div className="flex gap-2">
                      <div className="relative flex-1">
                        <input
                          type="text"
                          value={couponInput}
                          onChange={(e) =>
                            setCouponInput(e.target.value.toUpperCase())
                          }
                          onKeyDown={(e) => {
                            if (e.key === "Enter") {
                              e.preventDefault();
                              handleApplyCoupon(e);
                            }
                          }}
                          placeholder="Promo code (e.g. DISCOUNT10)"
                          disabled={isApplyingCoupon}
                          className="w-full pl-8.5 pr-3 py-2.5 rounded-xl bg-slate-50 border border-slate-200 text-xs font-semibold uppercase text-[#0D1B2A] placeholder:normal-case placeholder:font-normal placeholder:text-slate-400 focus:outline-none focus:border-[#2196F3] focus:bg-white transition-all disabled:opacity-60"
                        />
                        <Tag className="w-3.5 h-3.5 text-slate-400 absolute left-2.5 top-1/2 -translate-y-1/2" />
                      </div>
                      <button
                        type="button"
                        onClick={handleApplyCoupon}
                        disabled={isApplyingCoupon || !couponInput.trim()}
                        className="px-4 py-2.5 rounded-xl bg-[#0D47A1] hover:bg-[#2196F3] text-white text-xs font-bold transition-all shadow-2xs disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer shrink-0 flex items-center justify-center min-w-[70px]"
                      >
                        {isApplyingCoupon ? (
                          <Loader2 className="w-3.5 h-3.5 animate-spin" />
                        ) : (
                          "Apply"
                        )}
                      </button>
                    </div>
                  )}
                </div>

                {/* Summary Calculations */}
                <div className="space-y-2 pt-3 border-t border-slate-100 text-xs text-slate-600">
                  <div className="flex justify-between">
                    <span>Subtotal</span>
                    <span className="font-semibold text-[#0D1B2A]">
                      {formatCurrency(subtotal)}
                    </span>
                  </div>

                  {discountAmount > 0 && (
                    <div className="flex justify-between text-emerald-600 font-semibold">
                      <span>Discount</span>
                      <span>-{formatCurrency(discountAmount)}</span>
                    </div>
                  )}

                  <div className="flex justify-between items-center">
                    <div className="flex flex-col">
                      <span>Shipping (GHN Express)</span>
                      {shippingError && (
                        <span className="text-[10px] text-amber-600">
                          {shippingError}
                        </span>
                      )}
                    </div>
                    <span className="font-semibold text-[#0D1B2A]">
                      {isCalculatingShipping ? (
                        <span className="inline-flex items-center gap-1 text-[#2196F3] text-[11px]">
                          <Loader2 className="w-3 h-3 animate-spin" />
                          Calculating...
                        </span>
                      ) : shippingFee !== null ? (
                        formatCurrency(shippingFee)
                      ) : (
                        <span className="text-slate-400 font-normal italic text-[11px]">
                          Select address
                        </span>
                      )}
                    </span>
                  </div>

                  <div className="flex justify-between text-base font-black text-[#0D1B2A] pt-3 border-t border-slate-200">
                    <span>Total Payment</span>
                    <span className="text-[#0D47A1]">
                      {formatCurrency(grandTotal)}
                    </span>
                  </div>
                </div>

                {/* Submit Order Button */}
                <button
                  type="submit"
                  disabled={isSubmitting || isCalculatingShipping}
                  className={`w-full py-4 px-6 rounded-2xl text-white text-sm font-bold flex items-center justify-center gap-2 transition-all shadow-md active:scale-98 disabled:bg-slate-300 cursor-pointer ${
                    paymentMethod === "MOMO"
                      ? "bg-[#A50064] hover:bg-[#850050]"
                      : "bg-[#2196F3] hover:bg-[#0D47A1]"
                  }`}
                >
                  {isSubmitting ? (
                    <div className="w-5 h-5 rounded-full border-2 border-white border-t-transparent animate-spin" />
                  ) : (
                    <>
                      <span>
                        {paymentMethod === "MOMO"
                          ? "Pay with MoMo"
                          : "Place Order Now"}
                      </span>
                      <ShieldCheck className="w-4 h-4" />
                    </>
                  )}
                </button>

                <p className="text-[11px] text-slate-400 text-center leading-relaxed">
                  By placing your order, you agree to GOshop&apos;s Terms of
                  Service and Privacy Policy.
                </p>
              </div>
            </div>
          </div>
        </form>

        {/* Modal Popup: Add / Edit Address */}
        {isAddressModalOpen && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-900/50 backdrop-blur-xs animate-in fade-in">
            <div
              className="bg-white rounded-2xl shadow-2xl border border-slate-200 max-w-lg w-full max-h-[90vh] flex flex-col overflow-hidden animate-in zoom-in-95"
              onClick={(e) => e.stopPropagation()}
            >
              {/* Modal Header */}
              <div className="px-6 py-4 border-b border-slate-100 flex items-center justify-between shrink-0">
                <div>
                  <h3 className="text-base font-bold text-[#0D1B2A]">
                    {editingAddressUuid
                      ? "Edit Shipping Address"
                      : "Add Shipping Address"}
                  </h3>
                  <p className="text-xs text-slate-500">
                    Enter contact information and delivery address
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => setIsAddressModalOpen(false)}
                  className="w-8 h-8 rounded-full hover:bg-slate-100 text-slate-400 hover:text-slate-600 flex items-center justify-center transition-colors cursor-pointer"
                >
                  <X className="w-4 h-4" />
                </button>
              </div>

              {/* Modal Form */}
              <form
                onSubmit={handleSaveAddress}
                className="flex-1 overflow-y-auto p-6 space-y-5"
              >
                {/* 1. Contact Information */}
                <div className="space-y-3">
                  <h4 className="text-xs font-bold text-[#0D47A1] uppercase tracking-wider">
                    Contact Information
                  </h4>
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                    <div className="space-y-1">
                      <label className="text-xs font-semibold text-slate-700">
                        Full Name <span className="text-rose-500">*</span>
                      </label>
                      <input
                        type="text"
                        required
                        value={modalName}
                        onChange={(e) => setModalName(e.target.value)}
                        placeholder="e.g. John Doe"
                        className="w-full px-3 py-2 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                      />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs font-semibold text-slate-700">
                        Phone Number <span className="text-rose-500">*</span>
                      </label>
                      <input
                        type="tel"
                        required
                        value={modalPhone}
                        onChange={(e) => setModalPhone(e.target.value)}
                        placeholder="e.g. 0901234567"
                        className="w-full px-3 py-2 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                      />
                    </div>
                  </div>

                  <div className="space-y-1">
                    <label className="text-xs font-semibold text-slate-700">
                      Address Label
                    </label>
                    <div className="flex items-center gap-2">
                      {["Home", "Office", "Other"].map((labelOption) => (
                        <button
                          key={labelOption}
                          type="button"
                          onClick={() => setModalLabel(labelOption)}
                          className={`px-3 py-1.5 rounded-lg text-xs font-semibold border transition-all cursor-pointer ${
                            modalLabel === labelOption
                              ? "border-[#2196F3] bg-[#E3F2FD] text-[#0D47A1]"
                              : "border-slate-200 bg-white text-slate-600 hover:bg-slate-50"
                          }`}
                        >
                          {labelOption}
                        </button>
                      ))}
                    </div>
                  </div>
                </div>

                {/* 2. Shipping Address */}
                <div className="space-y-3 pt-4 border-t border-slate-100">
                  <div className="flex items-center justify-between">
                    <h4 className="text-xs font-bold text-[#0D47A1] uppercase tracking-wider">
                      Shipping Address
                    </h4>
                    <span className="text-[10px] text-slate-400">
                      Powered by GHN
                    </span>
                  </div>

                  <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                    {/* Province */}
                    <div className="space-y-1">
                      <label className="text-xs font-semibold text-slate-700">
                        City / Province <span className="text-rose-500">*</span>
                      </label>
                      <select
                        required
                        value={modalProvinceId || ""}
                        onChange={handleModalProvinceChange}
                        disabled={isLoadingProvinces}
                        className="w-full px-2.5 py-2 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3] cursor-pointer"
                      >
                        <option value="">
                          {isLoadingProvinces ? "Loading..." : "-- Select --"}
                        </option>
                        {provinces.map((p) => (
                          <option key={p.province_id} value={p.province_id}>
                            {p.province_name}
                          </option>
                        ))}
                      </select>
                    </div>

                    {/* District */}
                    <div className="space-y-1">
                      <label className="text-xs font-semibold text-slate-700">
                        District <span className="text-rose-500">*</span>
                      </label>
                      <select
                        required
                        value={modalDistrictId || ""}
                        onChange={handleModalDistrictChange}
                        disabled={!modalProvinceId || isLoadingModalDistricts}
                        className="w-full px-2.5 py-2 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3] cursor-pointer"
                      >
                        <option value="">
                          {isLoadingModalDistricts
                            ? "Loading..."
                            : !modalProvinceId
                              ? "Select province"
                              : "-- Select --"}
                        </option>
                        {modalDistricts.map((d) => (
                          <option key={d.district_id} value={d.district_id}>
                            {d.district_name}
                          </option>
                        ))}
                      </select>
                    </div>

                    {/* Ward */}
                    <div className="space-y-1">
                      <label className="text-xs font-semibold text-slate-700">
                        Ward <span className="text-rose-500">*</span>
                      </label>
                      <select
                        required
                        value={modalWardCode || ""}
                        onChange={handleModalWardChange}
                        disabled={!modalDistrictId || isLoadingModalWards}
                        className="w-full px-2.5 py-2 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3] cursor-pointer"
                      >
                        <option value="">
                          {isLoadingModalWards
                            ? "Loading..."
                            : !modalDistrictId
                              ? "Select district"
                              : "-- Select --"}
                        </option>
                        {modalWards.map((w) => (
                          <option key={w.ward_code} value={w.ward_code}>
                            {w.ward_name}
                          </option>
                        ))}
                      </select>
                    </div>
                  </div>

                  {/* Street Address */}
                  <div className="space-y-1">
                    <label className="text-xs font-semibold text-slate-700">
                      Street Address <span className="text-rose-500">*</span>
                    </label>
                    <input
                      type="text"
                      required
                      value={modalAddress}
                      onChange={(e) => setModalAddress(e.target.value)}
                      placeholder="Apartment, suite, house number, street name"
                      className="w-full px-3 py-2 rounded-xl bg-slate-50 border border-slate-200 text-xs text-[#0D1B2A] focus:outline-none focus:border-[#2196F3]"
                    />
                  </div>

                  {/* Default Checkbox */}
                  <div className="pt-1">
                    <label className="inline-flex items-center gap-2 cursor-pointer text-xs text-slate-700 font-medium select-none">
                      <input
                        type="checkbox"
                        checked={modalIsDefault}
                        onChange={(e) => setModalIsDefault(e.target.checked)}
                        className="rounded border-slate-300 text-[#2196F3] focus:ring-[#2196F3]"
                      />
                      <span>Set as default shipping address</span>
                    </label>
                  </div>
                </div>

                {/* Actions */}
                <div className="pt-4 border-t border-slate-100 flex items-center justify-end gap-2.5 shrink-0">
                  <button
                    type="button"
                    onClick={() => setIsAddressModalOpen(false)}
                    className="px-4 py-2 rounded-xl border border-slate-200 text-slate-700 text-xs font-semibold hover:bg-slate-50 transition-colors cursor-pointer"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={isSavingAddress}
                    className="px-5 py-2 rounded-xl bg-[#2196F3] text-white text-xs font-bold hover:bg-[#0D47A1] transition-all shadow-xs flex items-center gap-1.5 cursor-pointer disabled:opacity-60"
                  >
                    {isSavingAddress ? (
                      <>
                        <Loader2 className="w-3.5 h-3.5 animate-spin" />
                        <span>Saving...</span>
                      </>
                    ) : (
                      <span>Save Address</span>
                    )}
                  </button>
                </div>
              </form>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

export default function CheckoutPage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-[70vh] flex items-center justify-center">
          <div className="w-8 h-8 rounded-full border-2 border-[#2196F3] border-t-transparent animate-spin" />
        </div>
      }
    >
      <CheckoutContent />
    </Suspense>
  );
}
