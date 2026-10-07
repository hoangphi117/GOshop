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
  ArrowRight,
  ShoppingCart,
  CheckCircle2,
  Loader2,
  MapPin,
  Tag,
  AlertCircle,
  AlertTriangle,
  CreditCard,
  Plus,
  Pencil,
  Trash2,
  X,
  Lock,
  Zap,
  Truck,
  Check,
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
    orderUuid?: string;
    paymentMethod: string;
    paymentFailed?: boolean;
    shippingFee: number;
    shippingAddress?: string;
  } | null>(null);
  const [retryingPayment, setRetryingPayment] = useState(false);

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
  const [couponError, setCouponError] = useState<string | null>(null);

  const handleApplyCoupon = async (
    e?: React.FormEvent | React.MouseEvent | React.KeyboardEvent,
  ) => {
    if (e) e.preventDefault();
    setCouponError(null);

    if (!couponInput.trim()) {
      setCouponError("Please enter a voucher or promo code.");
      return;
    }

    const code = couponInput.trim().toUpperCase();

    if (!accessToken) {
      setCouponError("Please sign in to apply promo codes.");
      return;
    }

    if (subtotal <= 0) {
      setCouponError("Please select valid items before applying a coupon.");
      return;
    }

    setIsApplyingCoupon(true);
    try {
      const res = await authFetch("/api/v1/coupons/preview", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          code,
          subtotal_amount: subtotal,
        }),
      });

      const json = await res.json().catch(() => null);

      if (res.ok && json?.data) {
        setAppliedCoupon({
          code: json.data.coupon_code || code,
          name: json.data.coupon_name || "Promotional Discount",
          discountAmount: json.data.discount_amount || 0,
        });
        setCouponError(null);
        setCouponInput("");
      } else {
        const msg = extractErrorMessage(json, "Invalid or expired promo code.");
        setCouponError(msg);
      }
    } catch {
      setCouponError("Network error while validating voucher.");
    } finally {
      setIsApplyingCoupon(false);
    }
  };

  const handleRemoveCoupon = () => {
    setAppliedCoupon(null);
    setCouponError(null);
  };

  // 1. Fetch Provinces & Saved Addresses on Mount
  useEffect(() => {
    const fetchProvinces = async () => {
      setIsLoadingProvinces(true);
      try {
        const res = await fetch(`${API_BASE_URL}/api/v1/shipping/provinces`, {
          headers: {
            "x-api-key": API_KEY,
          },
        });
        if (res.ok) {
          const json = await res.json();
          setProvinces(json?.data || []);
        }
      } catch {
        // Silently ignore background shipping fetch failure
      } finally {
        setIsLoadingProvinces(false);
      }
    };

    fetchProvinces();
  }, []);

  // 2. Address Cascading Handlers
  const applySavedAddress = (addr: UserAddress) => {
    setSelectedSavedAddressUuid(addr.uuid);
    setFullName(addr.name || user?.fullName || "");
    setPhone(addr.phone || "");
    setAddress(addr.address || "");
    setSelectedProvinceId(addr.province_id || null);
    setSelectedProvinceName(addr.province_name || "");
    setSelectedDistrictId(addr.district_id || null);
    setSelectedDistrictName(addr.district_name || "");
    setSelectedWardCode(addr.ward_code || "");
    setSelectedWardName(addr.ward_name || "");
  };

  const fetchSavedAddresses = useCallback(async () => {
    if (!accessToken) return;
    try {
      const res = await authFetch("/api/v1/addresses");
      if (res.ok) {
        const json = await res.json();
        const addrs: UserAddress[] = json?.data || [];
        setSavedAddresses(addrs);

        // Auto select default address or first address
        if (addrs.length > 0) {
          const defaultAddr = addrs.find((a) => a.is_default) || addrs[0];
          applySavedAddress(defaultAddr);
        }
      }
    } catch {
      // Ignore background address load errors
    }
  }, [accessToken]);

  useEffect(() => {
    fetchSavedAddresses();
  }, [fetchSavedAddresses]);

  const loadModalWards = async (
    distId: number,
    targetWardCode?: string,
    targetWardName?: string,
  ) => {
    setIsLoadingModalWards(true);
    try {
      const res = await fetch(
        `${API_BASE_URL}/api/v1/shipping/wards?district_id=${distId}`,
        {
          headers: {
            "x-api-key": API_KEY,
          },
        },
      );
      if (res.ok) {
        const json = await res.json();
        const wardList: Ward[] = json?.data || [];
        setModalWards(wardList);

        if (targetWardCode) {
          const foundWard = wardList.find(
            (w) => w.ward_code === targetWardCode,
          );
          if (foundWard) {
            setModalWardCode(targetWardCode);
            setModalWardName(targetWardName || foundWard.ward_name);
          }
        }
      }
    } catch {
      // Silently ignore
    } finally {
      setIsLoadingModalWards(false);
    }
  };

  const loadModalDistricts = async (
    provId: number,
    targetDistrictId?: number,
    targetWardCode?: string,
    targetWardName?: string,
  ) => {
    setIsLoadingModalDistricts(true);
    try {
      const res = await fetch(
        `${API_BASE_URL}/api/v1/shipping/districts?province_id=${provId}`,
        {
          headers: {
            "x-api-key": API_KEY,
          },
        },
      );
      if (res.ok) {
        const json = await res.json();
        const distList: District[] = json?.data || [];
        setModalDistricts(distList);

        if (targetDistrictId) {
          const foundDist = distList.find(
            (d) => d.district_id === targetDistrictId,
          );
          if (foundDist) {
            setModalDistrictId(targetDistrictId);
            setModalDistrictName(foundDist.district_name);
            loadModalWards(targetDistrictId, targetWardCode, targetWardName);
          }
        }
      }
    } catch {
      // Silently ignore
    } finally {
      setIsLoadingModalDistricts(false);
    }
  };

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

  // 3. Save / Update Address
  const handleSaveAddress = async (e: React.FormEvent) => {
    e.preventDefault();
    if (
      !modalName.trim() ||
      !modalPhone.trim() ||
      !modalAddress.trim() ||
      !modalProvinceId ||
      !modalDistrictId ||
      !modalWardCode
    ) {
      toast.error("Please fill in all required shipping address fields.");
      return;
    }

    setIsSavingAddress(true);
    try {
      const fullAddressStr = `${modalAddress.trim()}, ${modalWardName}, ${modalDistrictName}, ${modalProvinceName}`;
      const payload = {
        label: modalLabel || "Home",
        name: modalName.trim(),
        phone: modalPhone.trim(),
        address: modalAddress.trim(),
        province_id: modalProvinceId,
        province_name: modalProvinceName,
        district_id: modalDistrictId,
        district_name: modalDistrictName,
        ward_code: modalWardCode,
        ward_name: modalWardName,
        full_address: fullAddressStr,
        is_default: modalIsDefault,
      };

      const url = editingAddressUuid
        ? `/api/v1/addresses/${editingAddressUuid}`
        : "/api/v1/addresses";
      const method = editingAddressUuid ? "PUT" : "POST";

      const res = await authFetch(url, {
        method,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });

      if (res.ok) {
        toast.success(
          editingAddressUuid
            ? "Shipping address updated."
            : "New shipping address added.",
        );
        setIsAddressModalOpen(false);
        await fetchSavedAddresses();
      } else {
        const errJson = await res.json().catch(() => null);
        const msg = extractErrorMessage(errJson, "Failed to save address.");
        toast.error(msg);
      }
    } catch {
      toast.error("Network error saving address.");
    } finally {
      setIsSavingAddress(false);
    }
  };

  // Delete Address
  const handleDeleteAddress = async (uuid: string, e: React.MouseEvent) => {
    e.stopPropagation();
    if (
      !window.confirm("Are you sure you want to remove this delivery address?")
    ) {
      return;
    }

    setIsDeletingAddressUuid(uuid);
    try {
      const res = await authFetch(`/api/v1/addresses/${uuid}`, {
        method: "DELETE",
      });
      if (!res.ok) {
        const errJson = await res.json().catch(() => null);
        const msg = extractErrorMessage(errJson, "Failed to delete address.");
        toast.error(msg);
        return;
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
        const res = await fetch(
          `${API_BASE_URL}/api/v1/shipping/calculate-fee`,
          {
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
          },
        );

        if (!isMounted) return;

        if (res.ok) {
          const json = await res.json();
          if (json?.data?.shipping_fee !== undefined) {
            setShippingFee(json.data.shipping_fee);
          }
        } else {
          // Fallback if carrier rate lookup encounters issue
          setShippingFee(30000);
          setShippingError(
            "Carrier rate unavailable. Applied standard delivery fee.",
          );
        }
      } catch {
        if (isMounted) {
          setShippingFee(30000);
          setShippingError("Network issue. Applied standard delivery fee.");
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

      const res = await authFetch("/api/v1/orders", {
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
        let paymentFailed = false;
        try {
          const paymentRes = await authFetch("/api/v1/payments/create-intent", {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
              "X-Idempotency-Key": `idem-pay-${createdOrderUuid}`,
            },
            body: JSON.stringify({
              order_uuid: createdOrderUuid,
              provider: "momo",
            }),
          }).catch(() => null);

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
            } else {
              paymentFailed = true;
            }
          } else {
            paymentFailed = true;
          }
        } catch {
          paymentFailed = true;
        }

        if (isBuyNow) {
          clearBuyNowItem();
        } else {
          removeSelectedItems();
        }
        setAppliedCoupon(null);
        setOrderPlaced({
          orderId: generatedId,
          orderUuid: createdOrderUuid,
          paymentMethod,
          paymentFailed,
          shippingFee: finalShippingFee,
          shippingAddress: fullShippingAddress,
        });
        if (!paymentFailed) {
          toast.success("Order placed successfully!");
        }
        return;
      }

      if (isBuyNow) {
        clearBuyNowItem();
      } else {
        removeSelectedItems();
      }
      setAppliedCoupon(null);
      setOrderPlaced({
        orderId: generatedId,
        orderUuid: createdOrderUuid,
        paymentMethod,
        paymentFailed: false,
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

  const handleRetryPayment = async (orderUuid?: string) => {
    if (!orderUuid) return;
    setRetryingPayment(true);
    try {
      const baseUrl =
        process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      const res = await authFetch(`${baseUrl}/api/v1/payments/create-intent`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Idempotency-Key": `idem-pay-${orderUuid}-${Date.now()}`,
        },
        body: JSON.stringify({
          order_uuid: orderUuid,
          provider: "momo",
        }),
      });

      if (res.ok) {
        const data = await res.json();
        const paymentUrl = data?.data?.payment_url;
        if (paymentUrl) {
          toast.success("Redirecting to MoMo payment gateway...");
          window.location.href = paymentUrl;
          return;
        }
      }

      const errJson = await res.json().catch(() => null);
      const errMsg = extractErrorMessage(
        errJson,
        "Payment gateway is temporarily unavailable. Please try again later.",
      );
      toast.error(errMsg);
    } catch {
      toast.error("Network error while connecting to payment gateway.");
    } finally {
      setRetryingPayment(false);
    }
  };

  if (!mounted) {
    return (
      <div className="min-h-[60vh] flex items-center justify-center bg-[#F8FAFC]">
        <Loader2 className="w-8 h-8 text-[#007D9C] animate-spin" />
      </div>
    );
  }

  if (!user || !accessToken) {
    return (
      <div className="min-h-[70vh] bg-[#F8FAFC] py-16 flex items-center justify-center">
        <div className="max-w-md w-full mx-auto px-4 text-center space-y-4 bg-white p-8 border border-[#E2E8F0] rounded-md shadow-xs">
          <div className="w-14 h-14 rounded-md bg-slate-100 text-[#007D9C] flex items-center justify-center mx-auto border border-[#E2E8F0]">
            <Lock className="w-6 h-6" />
          </div>
          <h1 className="font-heading text-xl font-bold text-[#0F172A]">
            Sign In Required
          </h1>
          <p className="text-xs text-[#64748B] leading-relaxed">
            Please sign in to your GOshop account to access your saved addresses
            and proceed with checkout.
          </p>
          <div className="pt-2 flex flex-col sm:flex-row items-center justify-center gap-3">
            <Link
              href="/login?redirect=/checkout"
              className="w-full sm:w-auto px-5 py-2.5 rounded-md bg-[#007D9C] text-white text-xs font-bold hover:bg-[#00657E] transition-colors shadow-xs text-center"
            >
              Sign In Now
            </Link>
            <Link
              href="/cart"
              className="w-full sm:w-auto px-5 py-2.5 rounded-md border border-[#E2E8F0] bg-white text-[#0F172A] text-xs font-semibold hover:bg-[#F8FAFC] transition-colors text-center"
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
      <div className="min-h-[70vh] bg-[#F8FAFC] py-16 flex items-center justify-center">
        <div className="max-w-md w-full mx-auto px-4 text-center space-y-4 bg-white p-8 border border-[#E2E8F0] rounded-md shadow-xs">
          <div className="w-14 h-14 rounded-md bg-slate-100 text-[#007D9C] flex items-center justify-center mx-auto border border-[#E2E8F0]">
            <ShoppingCart className="w-6 h-6" />
          </div>
          <h1 className="font-heading text-xl font-bold text-[#0F172A]">
            No Items Selected for Checkout
          </h1>
          <p className="text-xs text-[#64748B] leading-relaxed">
            Please choose the products you wish to purchase from your shopping
            cart.
          </p>
          <div className="pt-2">
            <Link
              href="/cart"
              className="inline-flex items-center gap-2 px-5 py-2.5 rounded-md bg-[#007D9C] text-white text-xs font-bold hover:bg-[#00657E] transition-colors shadow-xs"
            >
              <ArrowLeft className="w-3.5 h-3.5" />
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
      <div className="bg-[#F8FAFC] min-h-[75vh] py-12 flex items-center justify-center">
        <div className="max-w-lg w-full mx-auto px-4 text-center space-y-5 bg-white p-6 sm:p-8 border border-[#E2E8F0] rounded-md shadow-xs">
          <div
            className={`w-14 h-14 rounded-md flex items-center justify-center mx-auto border ${
              orderPlaced.paymentFailed
                ? "bg-amber-50 text-amber-600 border-amber-200"
                : "bg-emerald-50 text-emerald-600 border-emerald-200"
            }`}
          >
            {orderPlaced.paymentFailed ? (
              <AlertTriangle className="w-7 h-7" />
            ) : (
              <CheckCircle2 className="w-7 h-7" />
            )}
          </div>
          <h1 className="font-heading text-2xl font-bold text-[#0F172A]">
            {orderPlaced.paymentFailed
              ? "Order Created (Payment Pending)"
              : "Thank You For Your Order!"}
          </h1>
          <p className="text-xs text-[#64748B] leading-relaxed">
            Your order{" "}
            <strong className="text-[#007D9C] font-mono">
              #{orderPlaced.orderId}
            </strong>{" "}
            has been placed successfully.
          </p>

          {orderPlaced.paymentFailed && (
            <div className="p-3.5 rounded-md bg-amber-50 border border-amber-200 text-amber-900 text-xs text-left space-y-1.5 shadow-2xs">
              <div className="flex items-center gap-1.5 font-bold">
                <AlertCircle className="w-4 h-4 text-amber-600 shrink-0" />
                <span>Payment pending (15-minute window)</span>
              </div>
              <p className="text-amber-700 leading-relaxed text-[11px]">
                We could not connect to the MoMo gateway automatically. You can
                retry paying now. Unpaid orders will automatically expire and
                release reserved stock after 15 minutes.
              </p>
            </div>
          )}

          <div className="p-4 rounded-md bg-[#F8FAFC] border border-[#E2E8F0] text-left space-y-2.5 text-xs">
            <div className="flex justify-between">
              <span className="text-[#64748B]">Payment Mode:</span>
              <span className="font-bold text-[#0F172A]">
                {orderPlaced.paymentMethod === "COD"
                  ? "Cash on Delivery (COD)"
                  : "MoMo E-Wallet"}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-[#64748B]">Delivery Destination:</span>
              <span
                className="font-medium text-[#0F172A] truncate max-w-xs"
                title={orderPlaced.shippingAddress}
              >
                {orderPlaced.shippingAddress ||
                  `${address}, ${selectedWardName}, ${selectedDistrictName}, ${selectedProvinceName}`}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-[#64748B]">Shipping Partner:</span>
              <span className="font-semibold text-emerald-700">
                GHN Express ({formatCurrency(orderPlaced.shippingFee)})
              </span>
            </div>
          </div>

          <div className="pt-2 flex flex-col sm:flex-row gap-3 justify-center">
            {orderPlaced.paymentFailed && orderPlaced.orderUuid && (
              <button
                type="button"
                disabled={retryingPayment}
                onClick={() => handleRetryPayment(orderPlaced.orderUuid)}
                className="px-5 py-2.5 rounded-md bg-[#F26522] hover:bg-[#D95316] text-white text-xs font-bold transition-colors shadow-xs flex items-center justify-center gap-2 cursor-pointer disabled:opacity-50"
              >
                {retryingPayment ? (
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                ) : (
                  <CreditCard className="w-3.5 h-3.5" />
                )}
                <span>Pay Now (MoMo)</span>
              </button>
            )}
            <Link
              href="/account/orders"
              className="px-5 py-2.5 rounded-md bg-[#007D9C] text-white text-xs font-bold hover:bg-[#00657E] transition-colors shadow-xs flex items-center justify-center gap-1.5"
            >
              <span>View My Orders</span>
              <ArrowRight className="w-3.5 h-3.5" />
            </Link>
            <Link
              href="/products"
              className="px-5 py-2.5 rounded-md bg-white border border-[#E2E8F0] text-[#0F172A] text-xs font-semibold hover:bg-[#F8FAFC] transition-colors flex items-center justify-center"
            >
              Continue Shopping
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="bg-[#F8FAFC] min-h-screen py-6">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 space-y-6">
        {/* Header & 3-Step Stepper Bar */}
        <div className="bg-white border border-[#E2E8F0] rounded-md p-4 sm:p-5 shadow-xs flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div className="space-y-1">
            <Link
              href={
                isBuyNow && buyNowItem
                  ? `/products/${buyNowItem.productUuid}`
                  : "/cart"
              }
              className="inline-flex items-center gap-1.5 text-xs font-semibold text-[#64748B] hover:text-[#007D9C] transition-colors"
            >
              <ArrowLeft className="w-3.5 h-3.5" />
              <span>{isBuyNow ? "Back to Product" : "Back to Cart"}</span>
            </Link>
            <div className="flex items-center gap-2.5 pt-1">
              <h1 className="font-heading text-xl sm:text-2xl font-bold text-[#0F172A] tracking-tight">
                Checkout
              </h1>
            </div>
          </div>

          {/* Stepper (1. Cart -> 2. Information & Payment -> 3. Order Placed) */}
          <div className="flex items-center gap-2 sm:gap-3 text-xs">
            <div className="flex items-center gap-1.5 text-[#007D9C] font-semibold">
              <div className="w-5 h-5 rounded-full bg-[#007D9C] text-white text-[10px] font-bold flex items-center justify-center">
                <Check className="w-3 h-3" />
              </div>
              <span className="hidden sm:inline">1. Cart</span>
            </div>
            <div className="w-6 h-px bg-[#007D9C]" />
            <div className="flex items-center gap-1.5 text-[#007D9C] font-bold">
              <div className="w-5 h-5 rounded-full bg-[#007D9C] text-white text-[10px] font-bold flex items-center justify-center">
                2
              </div>
              <span>2. Information & Payment</span>
            </div>
            <div className="w-6 h-px bg-[#E2E8F0]" />
            <div className="flex items-center gap-1.5 text-[#94A3B8] font-medium">
              <div className="w-5 h-5 rounded-full bg-slate-100 text-[#94A3B8] text-[10px] font-bold flex items-center justify-center border border-slate-200">
                3
              </div>
              <span className="hidden sm:inline">3. Confirmation</span>
            </div>
          </div>
        </div>

        {/* Not Logged In Warning Banner */}
        {!accessToken && (
          <div className="p-4 rounded-md bg-amber-50 border border-amber-200 flex items-center justify-between gap-4">
            <div className="flex items-center gap-2.5 text-xs text-amber-800 font-medium">
              <AlertCircle className="w-4 h-4 text-amber-600 shrink-0" />
              <span>
                Please sign in to access your saved addresses, apply discount
                coupons, and complete order placement.
              </span>
            </div>
            <Link
              href="/login?redirect=/checkout"
              className="shrink-0 px-3.5 py-1.5 rounded-md bg-amber-600 text-white text-xs font-bold hover:bg-amber-700 transition-colors shadow-xs"
            >
              Sign In
            </Link>
          </div>
        )}

        <form onSubmit={handleSubmitOrder}>
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-6 lg:gap-8 items-start">
            {/* Left Column: Form Details (7 cols) */}
            <div className="lg:col-span-7 space-y-6">
              {/* Step 1: Customer Contact Information */}
              <div className="bg-white border border-[#E2E8F0] rounded-md p-5 sm:p-6 shadow-xs space-y-4">
                <div className="flex items-center justify-between pb-3 border-b border-[#E2E8F0]">
                  <div className="flex items-center gap-2">
                    <span className="font-heading text-xs font-bold text-[#007D9C] bg-slate-100 px-2 py-0.5 rounded-md border border-slate-200">
                      Step 1
                    </span>
                    <h2 className="font-heading text-sm sm:text-base font-bold text-[#0F172A]">
                      Customer Information
                    </h2>
                  </div>
                  <span className="text-[11px] text-[#64748B]">
                    Logged in as{" "}
                    <strong>{user?.email || user?.fullName}</strong>
                  </span>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-xs">
                  <div className="p-3 bg-[#F8FAFC] border border-[#E2E8F0] rounded-md">
                    <span className="text-[#64748B] block text-[11px]">
                      Recipient Name
                    </span>
                    <span className="font-bold text-[#0F172A] mt-0.5 block">
                      {fullName || user?.fullName || "Not provided"}
                    </span>
                  </div>
                  <div className="p-3 bg-[#F8FAFC] border border-[#E2E8F0] rounded-md">
                    <span className="text-[#64748B] block text-[11px]">
                      Contact Phone
                    </span>
                    <span className="font-bold text-[#0F172A] mt-0.5 block">
                      {phone || "Selected from address"}
                    </span>
                  </div>
                </div>
              </div>

              {/* Step 2: Shipping & Delivery Destination */}
              <div className="bg-white border border-[#E2E8F0] rounded-md p-5 sm:p-6 shadow-xs space-y-4">
                <div className="flex items-center justify-between pb-3 border-b border-[#E2E8F0]">
                  <div className="flex items-center gap-2">
                    <span className="font-heading text-xs font-bold text-[#007D9C] bg-slate-100 px-2 py-0.5 rounded-md border border-slate-200">
                      Step 2
                    </span>
                    <h2 className="font-heading text-sm sm:text-base font-bold text-[#0F172A]">
                      Shipping Address
                    </h2>
                  </div>
                  {savedAddresses.length > 0 && (
                    <button
                      type="button"
                      onClick={openCreateAddressModal}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-[#007D9C] text-white text-xs font-semibold hover:bg-[#00657E] transition-colors shadow-xs cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#007D9C]"
                    >
                      <Plus className="w-3.5 h-3.5" />
                      <span>Add New Address</span>
                    </button>
                  )}
                </div>

                {/* Saved Address List */}
                {savedAddresses.length > 0 ? (
                  <div className="space-y-2.5">
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
                          className={`p-3.5 rounded-md border transition-colors cursor-pointer relative flex flex-col sm:flex-row sm:items-center justify-between gap-3 ${
                            isSelected
                              ? "border-2 border-[#007D9C] bg-[#F0F9FB]"
                              : "border-[#E2E8F0] bg-white hover:border-[#CBD5E1]"
                          }`}
                        >
                          <div className="flex items-start gap-3 flex-1 min-w-0">
                            {/* Radio Icon */}
                            <div
                              className={`w-4 h-4 rounded-full border mt-0.5 shrink-0 flex items-center justify-center transition-colors ${
                                isSelected
                                  ? "border-[#007D9C] bg-[#007D9C]"
                                  : "border-slate-300 bg-white"
                              }`}
                            >
                              {isSelected && (
                                <div className="w-1.5 h-1.5 rounded-full bg-white" />
                              )}
                            </div>

                            <div className="space-y-1 min-w-0 flex-1">
                              <div className="flex items-center gap-2 flex-wrap">
                                <span className="text-xs font-bold text-[#0F172A]">
                                  {addr.name}
                                </span>
                                <span className="text-xs text-[#64748B]">
                                  {addr.phone}
                                </span>
                                {addr.label && (
                                  <span className="text-[10px] font-medium px-2 py-0.5 rounded-md bg-slate-100 text-[#64748B] border border-slate-200">
                                    {addr.label}
                                  </span>
                                )}
                                {addr.is_default && (
                                  <span className="text-[10px] font-bold px-2 py-0.5 rounded-md bg-slate-100 text-[#0F172A] border border-slate-200">
                                    Default
                                  </span>
                                )}
                                {!hasGhnData && (
                                  <span className="text-[10px] font-medium px-2 py-0.5 rounded-md bg-slate-100 text-[#64748B] border border-slate-200">
                                    Incomplete Area (Click Edit)
                                  </span>
                                )}
                              </div>

                              <p className="text-xs text-[#64748B] leading-relaxed">
                                {addr.full_address || addr.address}
                              </p>
                            </div>
                          </div>

                          {/* Action Buttons: Edit & Delete */}
                          <div className="flex items-center gap-1.5 shrink-0 self-end sm:self-center pt-2 sm:pt-0 border-t sm:border-t-0 border-[#E2E8F0] w-full sm:w-auto justify-end">
                            <button
                              type="button"
                              onClick={(e) => openEditAddressModal(addr, e)}
                              className="inline-flex items-center gap-1 px-2 py-1 rounded-md border border-[#E2E8F0] text-[#64748B] hover:text-[#0F172A] hover:bg-[#F8FAFC] text-xs font-medium transition-colors cursor-pointer"
                              title="Edit address"
                            >
                              <Pencil className="w-3 h-3" />
                              <span>Edit</span>
                            </button>
                            <button
                              type="button"
                              disabled={isDeleting}
                              onClick={(e) => handleDeleteAddress(addr.uuid, e)}
                              className="inline-flex items-center gap-1 px-2 py-1 rounded-md border border-[#E2E8F0] text-[#64748B] hover:text-rose-600 hover:bg-rose-50 hover:border-rose-200 text-xs font-medium transition-colors disabled:opacity-50 cursor-pointer"
                              title="Delete address"
                            >
                              {isDeleting ? (
                                <Loader2 className="w-3 h-3 animate-spin" />
                              ) : (
                                <Trash2 className="w-3 h-3" />
                              )}
                              <span>Delete</span>
                            </button>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                ) : (
                  <div className="p-6 text-center rounded-md border-2 border-dashed border-[#E2E8F0] bg-[#F8FAFC] flex flex-col items-center justify-center gap-2.5">
                    <div className="w-10 h-10 rounded-md bg-slate-100 text-[#64748B] flex items-center justify-center border border-slate-200">
                      <MapPin className="w-5 h-5" />
                    </div>
                    <div className="space-y-0.5">
                      <h3 className="text-xs font-bold text-[#0F172A]">
                        No Shipping Address Found
                      </h3>
                      <p className="text-[11px] text-[#64748B] max-w-sm">
                        You don&apos;t have any saved delivery address yet.
                        Please add a shipping address to proceed.
                      </p>
                    </div>
                    <button
                      type="button"
                      onClick={openCreateAddressModal}
                      className="mt-1 inline-flex items-center gap-1.5 px-4 py-2 rounded-md bg-[#007D9C] text-white text-xs font-bold hover:bg-[#00657E] transition-colors shadow-xs cursor-pointer"
                    >
                      <Plus className="w-3.5 h-3.5" />
                      <span>Add Shipping Address</span>
                    </button>
                  </div>
                )}

                {/* Delivery Note */}
                <div className="space-y-1 pt-2 border-t border-[#E2E8F0]">
                  <label className="text-xs font-semibold text-[#0F172A]">
                    Delivery Instructions / Special Notes{" "}
                    <span className="text-[#64748B] font-normal">
                      (Optional)
                    </span>
                  </label>
                  <textarea
                    rows={2}
                    value={note}
                    onChange={(e) => setNote(e.target.value)}
                    placeholder="e.g. Call 10 minutes before delivery, drop at apartment reception desk"
                    className="w-full px-3 py-2 rounded-md bg-white border border-[#E2E8F0] text-xs text-[#0F172A] placeholder:text-[#94A3B8] focus:outline-none focus:border-[#007D9C] focus:ring-1 focus:ring-[#007D9C] resize-none"
                  />
                </div>
              </div>

              {/* Step 3: Shipping Carrier & Delivery Tier */}
              <div className="bg-white border border-[#E2E8F0] rounded-md p-5 sm:p-6 shadow-xs space-y-3">
                <div className="flex items-center gap-2 pb-3 border-b border-[#E2E8F0]">
                  <span className="font-heading text-xs font-bold text-[#007D9C] bg-slate-100 px-2 py-0.5 rounded-md border border-slate-200">
                    Step 3
                  </span>
                  <h2 className="font-heading text-sm sm:text-base font-bold text-[#0F172A]">
                    Shipping Method
                  </h2>
                </div>

                <div className="p-3.5 rounded-md border-2 border-[#007D9C] bg-[#F0F9FB] flex items-center justify-between gap-3">
                  <div className="flex items-start gap-2.5">
                    <Truck className="w-4 h-4 text-[#007D9C] mt-0.5 shrink-0" />
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="text-xs font-bold text-[#0F172A]">
                          GHN Express Courier
                        </span>
                        <span className="text-[10px] font-semibold px-2 py-0.2 rounded-md bg-slate-100 text-[#64748B] border border-slate-200">
                          2–3 Days
                        </span>
                      </div>
                      <p className="text-[11px] text-[#64748B] mt-0.5">
                        Standard nationwide ground delivery with live package
                        tracking.
                      </p>
                    </div>
                  </div>

                  <div className="text-right shrink-0">
                    {isCalculatingShipping ? (
                      <span className="inline-flex items-center gap-1 text-[#007D9C] text-xs font-semibold">
                        <Loader2 className="w-3 h-3 animate-spin" />
                        Calculating...
                      </span>
                    ) : shippingFee !== null ? (
                      <span className="font-heading text-sm font-bold text-[#0F172A]">
                        {formatCurrency(shippingFee)}
                      </span>
                    ) : (
                      <span className="text-[11px] text-[#64748B] italic">
                        Select destination
                      </span>
                    )}
                  </div>
                </div>
              </div>

              {/* Step 4: Payment Method Selection */}
              <div className="bg-white border border-[#E2E8F0] rounded-md p-5 sm:p-6 shadow-xs space-y-3">
                <div className="flex items-center gap-2 pb-3 border-b border-[#E2E8F0]">
                  <span className="font-heading text-xs font-bold text-[#007D9C] bg-slate-100 px-2 py-0.5 rounded-md border border-slate-200">
                    Step 4
                  </span>
                  <h2 className="font-heading text-sm sm:text-base font-bold text-[#0F172A]">
                    Payment Method
                  </h2>
                </div>

                <div className="space-y-2.5">
                  {/* COD */}
                  <label
                    onClick={() => setPaymentMethod("COD")}
                    className={`flex items-start gap-3 p-3.5 rounded-md border cursor-pointer transition-colors ${
                      paymentMethod === "COD"
                        ? "border-2 border-[#007D9C] bg-[#F0F9FB]"
                        : "border-[#E2E8F0] bg-white hover:border-[#CBD5E1]"
                    }`}
                  >
                    <input
                      type="radio"
                      name="payment"
                      checked={paymentMethod === "COD"}
                      onChange={() => setPaymentMethod("COD")}
                      className="mt-0.5 text-[#007D9C] focus:ring-[#007D9C]"
                    />
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <Banknote className="w-4 h-4 text-[#007D9C]" />
                        <span className="text-xs font-bold text-[#0F172A]">
                          Cash on Delivery (COD)
                        </span>
                        <span className="text-[10px] font-semibold px-2 py-0.2 rounded-md bg-slate-100 text-[#64748B] border border-slate-200">
                          Pay upon Receipt
                        </span>
                      </div>
                      <p className="text-[11px] text-[#64748B] mt-0.5">
                        Pay in cash upon physical delivery by the courier
                        representative.
                      </p>
                    </div>
                  </label>

                  {/* MoMo E-Wallet */}
                  <label
                    onClick={() => setPaymentMethod("MOMO")}
                    className={`flex items-start gap-3 p-3.5 rounded-md border cursor-pointer transition-colors ${
                      paymentMethod === "MOMO"
                        ? "border-2 border-[#007D9C] bg-[#F0F9FB]"
                        : "border-[#E2E8F0] bg-white hover:border-[#CBD5E1]"
                    }`}
                  >
                    <input
                      type="radio"
                      name="payment"
                      checked={paymentMethod === "MOMO"}
                      onChange={() => setPaymentMethod("MOMO")}
                      className="mt-0.5 text-[#007D9C] focus:ring-[#007D9C]"
                    />
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <div className="w-4 h-4 rounded-xs bg-[#A50064] text-white font-bold text-[9px] flex items-center justify-center shrink-0">
                          M
                        </div>
                        <span className="text-xs font-bold text-[#0F172A]">
                          MoMo E-Wallet & Bank QR
                        </span>
                        <span className="text-[10px] font-semibold px-2 py-0.2 rounded-md bg-slate-100 text-[#64748B] border border-slate-200">
                          Instant Online
                        </span>
                      </div>
                      <p className="text-[11px] text-[#64748B] mt-0.5">
                        Instant QR code payment or seamless in-app confirmation
                        with MoMo.
                      </p>
                    </div>
                  </label>
                </div>
              </div>
            </div>

            {/* Right Column: Order Items & Summary (5 cols) */}
            <div className="lg:col-span-5 space-y-4">
              <div className="bg-white border border-[#E2E8F0] rounded-md p-5 sm:p-6 shadow-xs space-y-4 sticky top-6">
                <div className="flex items-center justify-between pb-3 border-b border-[#E2E8F0]">
                  <h2 className="font-heading text-sm sm:text-base font-bold text-[#0F172A]">
                    Order Summary
                  </h2>
                  <span className="text-xs font-semibold text-[#64748B] bg-slate-100 px-2 py-0.5 rounded-md border border-slate-200">
                    {selectedItems.length}{" "}
                    {selectedItems.length === 1 ? "Item" : "Items"}
                  </span>
                </div>

                {/* Items List */}
                <div className="max-h-64 overflow-y-auto divide-y divide-[#E2E8F0] pr-1 space-y-1">
                  {selectedItems.map((item) => (
                    <div
                      key={item.id}
                      className="py-2.5 flex items-center gap-3"
                    >
                      <div className="w-12 h-12 rounded-md bg-[#F8FAFC] overflow-hidden shrink-0 border border-[#E2E8F0] flex items-center justify-center p-1">
                        {item.image ? (
                          <img
                            src={getImageUrl(item.image)}
                            alt={item.name}
                            className="w-full h-full object-contain"
                          />
                        ) : (
                          <ShoppingCart className="w-4 h-4 text-[#94A3B8]" />
                        )}
                      </div>

                      <div className="flex-1 min-w-0">
                        <h4 className="text-xs font-bold text-[#0F172A] truncate">
                          {item.name}
                        </h4>
                        {(item.selectedSize || item.selectedColor) && (
                          <p className="text-[10px] text-[#64748B] mt-0.5">
                            {item.selectedSize && `Size: ${item.selectedSize}`}
                            {item.selectedSize && item.selectedColor && " • "}
                            {item.selectedColor &&
                              `Color: ${item.selectedColor}`}
                          </p>
                        )}
                        <p className="text-[11px] text-[#64748B]">
                          Qty: {item.quantity} × {formatCurrency(item.price)}
                        </p>
                      </div>

                      <span className="text-xs font-bold text-[#0F172A] font-heading">
                        {formatCurrency(item.price * item.quantity)}
                      </span>
                    </div>
                  ))}
                </div>

                {/* Promo / Coupon Code Section */}
                <div className="pt-3 border-t border-[#E2E8F0] space-y-2">
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-bold text-[#0F172A] flex items-center gap-1.5">
                      <Tag className="w-3.5 h-3.5 text-[#007D9C]" />
                      Promo & Voucher Code
                    </span>
                    {appliedCoupon && (
                      <span className="text-[10px] font-semibold px-2 py-0.5 rounded-md bg-slate-100 text-[#64748B] border border-slate-200">
                        1 Applied
                      </span>
                    )}
                  </div>

                  {appliedCoupon ? (
                    <div className="p-2.5 rounded-md bg-[#F8FAFC] border border-[#E2E8F0] text-xs flex items-center justify-between transition-all">
                      <div className="flex items-center gap-2 min-w-0">
                        <div className="w-6 h-6 rounded-md bg-[#007D9C] text-white flex items-center justify-center shrink-0">
                          <Tag className="w-3 h-3" />
                        </div>
                        <div className="min-w-0">
                          <div className="flex items-center gap-1.5">
                            <span className="font-bold text-[#0F172A] font-mono uppercase">
                              {appliedCoupon.code}
                            </span>
                            <span className="text-emerald-700 font-bold">
                              -{formatCurrency(discountAmount)}
                            </span>
                          </div>
                          {appliedCoupon.name && (
                            <p className="text-[10px] text-[#64748B] truncate">
                              {appliedCoupon.name}
                            </p>
                          )}
                        </div>
                      </div>
                      <button
                        type="button"
                        onClick={handleRemoveCoupon}
                        className="p-1 rounded-md text-[#64748B] hover:text-rose-600 hover:bg-rose-50 transition-colors cursor-pointer shrink-0"
                        title="Remove voucher"
                      >
                        <X className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  ) : (
                    <div className="space-y-1.5">
                      <div className="flex gap-2">
                        <div className="relative flex-1">
                          <input
                            type="text"
                            value={couponInput}
                            onChange={(e) => {
                              setCouponInput(e.target.value.toUpperCase());
                              if (couponError) setCouponError(null);
                            }}
                            onKeyDown={(e) => {
                              if (e.key === "Enter") {
                                e.preventDefault();
                                handleApplyCoupon(e);
                              }
                            }}
                            placeholder="Promo code (e.g. GOPHER10)"
                            disabled={isApplyingCoupon}
                            className={`w-full pl-8 pr-2.5 py-2 rounded-md bg-white border text-xs font-semibold uppercase text-[#0F172A] placeholder:normal-case placeholder:font-normal placeholder:text-[#94A3B8] focus:outline-none focus:ring-1 transition-colors disabled:opacity-60 ${
                              couponError
                                ? "border-rose-400 focus:border-rose-500 focus:ring-rose-500"
                                : "border-[#E2E8F0] focus:border-[#007D9C] focus:ring-[#007D9C]"
                            }`}
                          />
                          <Tag className="w-3.5 h-3.5 text-[#94A3B8] absolute left-2.5 top-1/2 -translate-y-1/2" />
                        </div>
                        <button
                          type="button"
                          onClick={handleApplyCoupon}
                          disabled={isApplyingCoupon || !couponInput.trim()}
                          className="px-3.5 py-2 rounded-md bg-[#007D9C] hover:bg-[#00657E] text-white text-xs font-bold transition-colors shadow-xs disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer shrink-0 flex items-center justify-center min-w-[65px]"
                        >
                          {isApplyingCoupon ? (
                            <Loader2 className="w-3.5 h-3.5 animate-spin" />
                          ) : (
                            "Apply"
                          )}
                        </button>
                      </div>
                      {couponError && (
                        <div className="flex items-center gap-1.5 text-[11px] text-rose-600 font-medium px-0.5">
                          <AlertCircle className="w-3.5 h-3.5 shrink-0" />
                          <span>{couponError}</span>
                        </div>
                      )}
                    </div>
                  )}
                </div>

                {/* Summary Calculations */}
                <div className="space-y-2 pt-3 border-t border-[#E2E8F0] text-xs text-[#64748B]">
                  <div className="flex justify-between">
                    <span>Subtotal</span>
                    <span className="font-semibold text-[#0F172A]">
                      {formatCurrency(subtotal)}
                    </span>
                  </div>

                  {discountAmount > 0 && (
                    <div className="flex justify-between text-emerald-700 font-semibold">
                      <span>Voucher Discount</span>
                      <span>-{formatCurrency(discountAmount)}</span>
                    </div>
                  )}

                  <div className="flex justify-between items-center">
                    <div className="flex flex-col">
                      <span>Shipping Fee (GHN)</span>
                      {shippingError && (
                        <span className="text-[10px] text-amber-700">
                          {shippingError}
                        </span>
                      )}
                    </div>
                    <span className="font-semibold text-[#0F172A]">
                      {isCalculatingShipping ? (
                        <span className="inline-flex items-center gap-1 text-[#007D9C] text-[11px]">
                          <Loader2 className="w-3 h-3 animate-spin" />
                          Calculating...
                        </span>
                      ) : shippingFee !== null ? (
                        formatCurrency(shippingFee)
                      ) : (
                        <span className="text-[#94A3B8] font-normal italic text-[11px]">
                          Select address
                        </span>
                      )}
                    </span>
                  </div>

                  <div className="flex justify-between items-baseline pt-3 border-t border-[#E2E8F0]">
                    <span className="font-heading text-sm font-bold text-[#0F172A]">
                      Total Amount
                    </span>
                    <span className="font-heading text-xl font-bold text-[#0F172A]">
                      {formatCurrency(grandTotal)}
                    </span>
                  </div>
                </div>

                {/* Primary CTA Submit Order Button */}
                <button
                  type="submit"
                  disabled={isSubmitting || isCalculatingShipping}
                  className="w-full h-12 px-5 rounded-md text-white text-xs sm:text-sm font-heading font-bold flex items-center justify-center gap-2 transition-colors shadow-xs cursor-pointer disabled:bg-slate-300 disabled:cursor-not-allowed bg-[#F26522] hover:bg-[#D95316] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#00ADD8]"
                >
                  {isSubmitting ? (
                    <div className="w-4 h-4 rounded-full border-2 border-white border-t-transparent animate-spin" />
                  ) : (
                    <>
                      <Lock className="w-4 h-4" />
                      <span>
                        {paymentMethod === "MOMO"
                          ? `Pay with MoMo (${formatCurrency(grandTotal)})`
                          : `Place Order Now (${formatCurrency(grandTotal)})`}
                      </span>
                    </>
                  )}
                </button>

                {/* Trust and Assurance Box */}
                <div className="p-3 bg-[#F8FAFC] border border-[#E2E8F0] rounded-md space-y-1.5 text-[11px] text-[#64748B]">
                  <div className="flex items-center gap-2">
                    <CheckCircle2 className="w-3.5 h-3.5 text-[#007D9C] shrink-0" />
                    <span>100% Genuine Certified Hardware & GoLang Gear</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <Truck className="w-3.5 h-3.5 text-[#007D9C] shrink-0" />
                    <span>30-Day Hassle-Free Returns & 2-Year Warranty</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </form>

        {/* Modal Popup: Add / Edit Address */}
        {isAddressModalOpen && (
          <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-xs animate-fade-in">
            <div
              className="bg-white rounded-md shadow-xl border border-[#E2E8F0] max-w-lg w-full max-h-[90vh] flex flex-col overflow-hidden"
              onClick={(e) => e.stopPropagation()}
            >
              {/* Modal Header */}
              <div className="px-5 py-4 border-b border-[#E2E8F0] flex items-center justify-between bg-[#F8FAFC] shrink-0">
                <div>
                  <h3 className="font-heading text-sm sm:text-base font-bold text-[#0F172A]">
                    {editingAddressUuid
                      ? "Edit Shipping Address"
                      : "Add Shipping Address"}
                  </h3>
                  <p className="text-xs text-[#64748B]">
                    Enter contact details and delivery location
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => setIsAddressModalOpen(false)}
                  className="w-8 h-8 rounded-md hover:bg-[#E2E8F0]/60 text-[#64748B] hover:text-[#0F172A] flex items-center justify-center transition-colors cursor-pointer"
                  aria-label="Close address dialog"
                >
                  <X className="w-4 h-4" />
                </button>
              </div>

              {/* Modal Form */}
              <form
                onSubmit={handleSaveAddress}
                className="flex-1 overflow-y-auto p-5 space-y-4"
              >
                {/* 1. Contact Information */}
                <div className="space-y-2.5">
                  <h4 className="text-xs font-bold text-[#0F172A] uppercase tracking-wider">
                    Recipient Contact
                  </h4>
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                    <div className="space-y-1">
                      <label className="text-xs font-semibold text-[#0F172A]">
                        Full Name <span className="text-rose-500">*</span>
                      </label>
                      <input
                        type="text"
                        required
                        value={modalName}
                        onChange={(e) => setModalName(e.target.value)}
                        placeholder="e.g. Alex Morgan"
                        className="w-full px-3 py-2 rounded-md bg-white border border-[#E2E8F0] text-xs text-[#0F172A] placeholder:text-[#94A3B8] focus:outline-none focus:border-[#007D9C] focus:ring-1 focus:ring-[#007D9C]"
                      />
                    </div>
                    <div className="space-y-1">
                      <label className="text-xs font-semibold text-[#0F172A]">
                        Phone Number <span className="text-rose-500">*</span>
                      </label>
                      <input
                        type="tel"
                        required
                        value={modalPhone}
                        onChange={(e) => setModalPhone(e.target.value)}
                        placeholder="e.g. 0901234567"
                        className="w-full px-3 py-2 rounded-md bg-white border border-[#E2E8F0] text-xs text-[#0F172A] placeholder:text-[#94A3B8] focus:outline-none focus:border-[#007D9C] focus:ring-1 focus:ring-[#007D9C]"
                      />
                    </div>
                  </div>

                  <div className="space-y-1">
                    <label className="text-xs font-semibold text-[#0F172A]">
                      Address Label
                    </label>
                    <div className="flex items-center gap-2">
                      {["Home", "Office", "Other"].map((labelOption) => (
                        <button
                          key={labelOption}
                          type="button"
                          onClick={() => setModalLabel(labelOption)}
                          className={`px-3 py-1.5 rounded-md text-xs font-semibold border transition-colors cursor-pointer ${
                            modalLabel === labelOption
                              ? "border-2 border-[#007D9C] bg-[#F0F9FB] text-[#007D9C]"
                              : "border-[#E2E8F0] bg-white text-[#64748B] hover:border-[#CBD5E1]"
                          }`}
                        >
                          {labelOption}
                        </button>
                      ))}
                    </div>
                  </div>
                </div>

                {/* 2. Shipping Address */}
                <div className="space-y-2.5 pt-3 border-t border-[#E2E8F0]">
                  <div className="flex items-center justify-between">
                    <h4 className="text-xs font-bold text-[#0F172A] uppercase tracking-wider">
                      Delivery Location
                    </h4>
                    <span className="text-[10px] text-[#64748B]">
                      Powered by GHN
                    </span>
                  </div>

                  <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                    {/* Province */}
                    <div className="space-y-1">
                      <label className="text-xs font-semibold text-[#0F172A]">
                        City / Province <span className="text-rose-500">*</span>
                      </label>
                      <select
                        required
                        value={modalProvinceId || ""}
                        onChange={handleModalProvinceChange}
                        disabled={isLoadingProvinces}
                        className="w-full px-2.5 py-2 rounded-md bg-white border border-[#E2E8F0] text-xs text-[#0F172A] focus:outline-none focus:border-[#007D9C] focus:ring-1 focus:ring-[#007D9C] cursor-pointer"
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
                      <label className="text-xs font-semibold text-[#0F172A]">
                        District <span className="text-rose-500">*</span>
                      </label>
                      <select
                        required
                        value={modalDistrictId || ""}
                        onChange={handleModalDistrictChange}
                        disabled={!modalProvinceId || isLoadingModalDistricts}
                        className="w-full px-2.5 py-2 rounded-md bg-white border border-[#E2E8F0] text-xs text-[#0F172A] focus:outline-none focus:border-[#007D9C] focus:ring-1 focus:ring-[#007D9C] cursor-pointer"
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
                      <label className="text-xs font-semibold text-[#0F172A]">
                        Ward <span className="text-rose-500">*</span>
                      </label>
                      <select
                        required
                        value={modalWardCode || ""}
                        onChange={handleModalWardChange}
                        disabled={!modalDistrictId || isLoadingModalWards}
                        className="w-full px-2.5 py-2 rounded-md bg-white border border-[#E2E8F0] text-xs text-[#0F172A] focus:outline-none focus:border-[#007D9C] focus:ring-1 focus:ring-[#007D9C] cursor-pointer"
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
                    <label className="text-xs font-semibold text-[#0F172A]">
                      Street Address <span className="text-rose-500">*</span>
                    </label>
                    <input
                      type="text"
                      required
                      value={modalAddress}
                      onChange={(e) => setModalAddress(e.target.value)}
                      placeholder="House number, building name, street address"
                      className="w-full px-3 py-2 rounded-md bg-white border border-[#E2E8F0] text-xs text-[#0F172A] placeholder:text-[#94A3B8] focus:outline-none focus:border-[#007D9C] focus:ring-1 focus:ring-[#007D9C]"
                    />
                  </div>

                  {/* Default Checkbox */}
                  <div className="pt-1">
                    <label className="inline-flex items-center gap-2 cursor-pointer text-xs text-[#0F172A] font-medium select-none">
                      <input
                        type="checkbox"
                        checked={modalIsDefault}
                        onChange={(e) => setModalIsDefault(e.target.checked)}
                        className="rounded-xs border-[#E2E8F0] text-[#007D9C] focus:ring-[#007D9C]"
                      />
                      <span>Set as default shipping address</span>
                    </label>
                  </div>
                </div>

                {/* Actions */}
                <div className="pt-3 border-t border-[#E2E8F0] flex items-center justify-end gap-2.5 shrink-0">
                  <button
                    type="button"
                    onClick={() => setIsAddressModalOpen(false)}
                    className="px-4 py-2 rounded-md border border-[#E2E8F0] text-[#0F172A] text-xs font-semibold hover:bg-[#F8FAFC] transition-colors cursor-pointer"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={isSavingAddress}
                    className="px-5 py-2 rounded-md bg-[#007D9C] text-white text-xs font-bold hover:bg-[#00657E] transition-colors shadow-xs flex items-center gap-1.5 cursor-pointer disabled:opacity-60"
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
        <div className="min-h-[70vh] flex items-center justify-center bg-[#F8FAFC]">
          <Loader2 className="w-8 h-8 text-[#007D9C] animate-spin" />
        </div>
      }
    >
      <CheckoutContent />
    </Suspense>
  );
}
